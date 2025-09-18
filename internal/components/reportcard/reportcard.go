package reportcard

import (
	"fmt"
	"math"
	"slices"
	"strings"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type TaskStatus int
type RowStatus int

// Option is used to set options in New. For example:
//
//	table := New(WithColumns([]Column{{Title: "ID", Width: 10}}))
type Option func(*Model)

const (
	NotStarted TaskStatus = iota
	Success
	Warning
	InProgress
	Failure // Failure will be assumed fatal
	Skip

	// Minimum width of the name column. (the name column is sized dynamically based on the longest name)
	minNameLength = 4

	// Default page size. Will be used if no page size is provided.
	defaultPageSize = 6

	RowLoading    RowStatus = iota // Waiting for a background task to complete. Outside the scope of the tasks.
	RowUserPrompt                  // User input required.
	RowNotStarted                  // Row not started.
	RowStarted                     // Row started and in progress.
	RowSuccess                     // Row successfully done.
	RowWarning                     // Row successfully done with warning(s).
	RowFailure                     // Row done with failure.
)

type TaskResultMsg struct {
	Status              TaskStatus
	Index               int              // Index is the index of the item in [Model].rows.
	CustomSuccessStatus CustomTaskStatus // CustomSuccessStatus contains options for replacing the normal success symbol.
	Output              any              // Output will be passed to the viewport renderer.
	ViewportName        string           // TODO: Implement Feature. Allows the caller to select a custom [Viewport] rendered to use. Will use the default viewport if value is empty or name could not be matched.
	ForceDefault        bool             // ForceDefault allows the caller to set this task's output as the default output when navigating to this row in the reportcard. Requires Output to be set.
}

type CustomTaskStatus struct {
	Message          string
	ForegroundColour string
	Alignment        lipgloss.Position // If provided, will override the default alignment for the column.
}

type Row struct {
	name              string
	tasks             []Task
	activeTaskIndex   int // The index of the task that is currently running.
	defaultTaskIndex  int // Show the output of the task with this index first when the user navigates to this row.
	selectedTaskIndex int // The index of the currently displayed task.
	status            RowStatus
	columnsReference  map[string]int
	prefixValues      []string
	message           string // TODO: Implement feature. message provides the user with message/information outside of the scope of specific tasks. E.g. user prompts or loading before start.
	promptUser        bool   // TODO: Implement feature.
	rowLevelLoading   bool   // TODO: Implement feature.
}

// NewRow returns a new [row].
//
// columns can't be empty or nil.
func NewRow(name string, tasks []Task, prefixValues []string, columns []Column) Row {
	if len(columns) == 0 {
		panic("columns can't be empty")
	}

	r := Row{
		status:            RowNotStarted,
		name:              name,
		tasks:             make([]Task, len(columns)),
		prefixValues:      prefixValues,
		columnsReference:  make(map[string]int, len(columns)),
		defaultTaskIndex:  -1,
		selectedTaskIndex: -1,
	}

	// r.Tasks contains tasks that should be run for the row, as well as, skipped tasks for the provided columns.
	for i, col := range columns {
		r.tasks[i].name = col.Name
		r.tasks[i].status = Skip
		r.columnsReference[col.Name] = i
	}

	for _, t := range tasks {
		if colI, ok := r.columnsReference[t.name]; ok {
			r.tasks[colI] = t
		} else {
			panic(fmt.Sprintf("task name: '%s' does not match a column name", t.name))
		}
	}

	return r
}

// start will be run on the row first. It will set the row's status to RowStarted if there are tasks to run.
// It will do nothing if the row is not in the correct state.
// It will set the row to done if there are no tasks to run.
func (r *Row) start() {
	if r.status != RowNotStarted {
		return
	}

	var hasWork bool

	for k := 0; k < len(r.tasks); k++ {
		if r.tasks[k].status == NotStarted {
			r.activeTaskIndex = k
			r.tasks[k].status = InProgress
			hasWork = true
			break
		}
	}

	if hasWork {
		r.status = RowStarted
	} else {
		r.status = RowSuccess
	}
}

// update updates the current active task using the provided [TaskResultMsg], and
// updates all proceeding tasks accordingly.
func (r *Row) update(msg TaskResultMsg) (PrevRowStatus, CurRowStatus RowStatus) {
	PrevRowStatus = r.status

	if r.status != RowStarted {
		// Can't send updates to a finished or not started row.
		CurRowStatus = r.status
		return
	}

	if msg.Status == InProgress || msg.Status == NotStarted {
		// the active task is already InProgress, thus don't run below || a task can't be returned to NotStarted.
		// This check prevents weird or unintended behaviours.
		CurRowStatus = r.status
		return
	}

	r.tasks[r.activeTaskIndex].status = msg.Status

	if msg.Status == Success {
		r.tasks[r.activeTaskIndex].customSuccessStatus = msg.CustomSuccessStatus
	}

	if msg.Output != nil {
		r.tasks[r.activeTaskIndex].hasOutput = true
		r.tasks[r.activeTaskIndex].viewportInputData = msg.Output
		r.setDefaultDisplayTask(r.activeTaskIndex, msg)
	}

	var rowNotDone bool

	for k := r.activeTaskIndex + 1; k < len(r.tasks); k++ {
		if r.tasks[k].status != NotStarted {
			continue
		}

		if msg.Status == Failure {
			if r.canTaskRunAnyway(k) {
				// If the original/active task failed, but the current
				// task is allowed to run anyway, change its status to
				// InProgress.
				r.tasks[k].status = InProgress
				r.activeTaskIndex = k
				rowNotDone = true
			} else {
				r.tasks[k].status = Skip
			}
		} else {
			r.tasks[k].status = InProgress
			rowNotDone = true
			r.activeTaskIndex = k
			break
		}
	}

	if !rowNotDone {
		// Row is done, there are no more tasks to run.
		for _, t := range r.tasks {
			switch t.status {
			case Failure:
				r.status = RowFailure
				CurRowStatus = RowFailure
				return
			case Warning:
				r.status = RowWarning
				CurRowStatus = RowWarning
				return
			}
		}

		r.status = RowSuccess
	}

	CurRowStatus = r.status
	return
}

// canTaskRunAnyway checks whether the task with the provided index can run despite
// the current active task failing.
func (r *Row) canTaskRunAnyway(taskIndex int) bool {
	return slices.Contains(r.tasks[taskIndex].shouldRunAnywayFor, r.tasks[r.activeTaskIndex].name)
}

// setSelected sets the selectedTaskIndex to the defaultTaskIndex.
//
// This will be run when "entering" a row.
func (r *Row) setSelected() {
	if !r.hasOutputToDisplay() {
		return
	}

	r.selectedTaskIndex = r.defaultTaskIndex
}

// getSelectedIndex returns the current selected task's index.
//
// Will return -1 if not set.
func (r *Row) getSelectedIndex() int {
	if !r.hasOutputToDisplay() {
		return -1
	}

	return r.selectedTaskIndex
}

// moveLeft selects the next task to the left of the currently selected task.
func (r *Row) moveLeft() {
	for k := r.selectedTaskIndex - 1; k >= 0; k-- {
		if r.tasks[k].hasOutput {
			r.selectedTaskIndex = k
			return
		}
	}
}

// moveRight selects the next task to the right of the currently selected task.
func (r *Row) moveRight() {
	for k := r.selectedTaskIndex + 1; k < len(r.tasks); k++ {
		if r.tasks[k].hasOutput {
			r.selectedTaskIndex = k
			return
		}
	}
}

// setDefaultDisplayTask sets the task on the row that will be shown first when "entering" the row.
func (r *Row) setDefaultDisplayTask(taskIndex int, msg TaskResultMsg) {
	// Prioritize force sets
	if msg.ForceDefault {
		r.defaultTaskIndex = taskIndex
		return
	}

	// Default task is not set OR
	// Set the latest successful task as the default task.
	if r.defaultTaskIndex < 0 || r.tasks[r.defaultTaskIndex].status == Success {
		r.defaultTaskIndex = taskIndex
		return
	}

	// Failures/Warnings is prioritized after force sets.
	if (msg.Status == Failure || msg.Status == Warning) && (r.tasks[r.defaultTaskIndex].status != Failure && r.tasks[r.defaultTaskIndex].status != Warning) {
		r.defaultTaskIndex = taskIndex
	}
}

// hasOutputToDisplay returns a bool indicating whether the row contains any tasks that have an output.
func (r *Row) hasOutputToDisplay() bool {
	// r.defaultTaskIndex is used because it will be set immediately after a task that returns output completes.
	return r.defaultTaskIndex >= 0
}

// shouldShowLeft returns whether there is a task to the left of the selected one with output.
func (r *Row) shouldShowLeft() bool {
	for k := r.selectedTaskIndex - 1; k >= 0; k-- {
		if r.tasks[k].hasOutput {
			return true
		}
	}

	return false
}

// shouldShowRight returns whether there is a task to the right of the selected one with output.
func (r *Row) shouldShowRight() bool {
	for k := r.selectedTaskIndex + 1; k < len(r.tasks); k++ {
		if r.tasks[k].hasOutput {
			return true
		}
	}

	return false
}

// getOutput returns the name of the viewport to use and the content that should be set.
func (r *Row) getOutput() (string, any) {
	if r.selectedTaskIndex < 0 {
		return "", nil
	}

	return r.tasks[r.selectedTaskIndex].viewportName, r.tasks[r.selectedTaskIndex].viewportInputData
}

// taskStatusSummary contains information uses to render the task in function Model.renderRow().
type taskStatusSummary struct {
	status     TaskStatus        // status of the task.
	content    string            // content will be the status symbol or custom-caller-provided symbol.
	useCustom  bool              // whether caller provided a custom symbol.
	colour     lipgloss.Color    // default colour for the status.
	alignment  lipgloss.Position // column alignment.
	isSelected bool              // task is selected.
}

// GetTaskStatusSummary returns a [taskStatusSummary] that is used for rendering the task's cell in the row.
func (r *Row) GetTaskStatusSummary(taskIndex int) taskStatusSummary {
	t := r.tasks[taskIndex]
	s := taskStatusSummary{status: t.status}

	if t.status == InProgress {
		return s
	}

	if t.status == Success && t.customSuccessStatus.Message != "" && t.customSuccessStatus.ForegroundColour != "" {
		s.content = t.customSuccessStatus.Message
		s.colour = lipgloss.Color(t.customSuccessStatus.ForegroundColour)
		s.alignment = t.customSuccessStatus.Alignment
		s.useCustom = true
	} else {
		s.content, s.colour = GetTaskStatusSymbols(t.status)
	}

	if taskIndex == r.selectedTaskIndex {
		s.isSelected = true
	}

	return s
}

type Task struct {
	status TaskStatus // status of the task.
	name   string     // name of the task.
	// shouldRunAnywayFor allows a task to "run" on the reportcard regardless of if a previous task
	// failed fatally. shouldRunAnywayFor is a map[string]bool, where the keys are column/task names and the
	// values are bools indicating whether the task should run for said task.
	shouldRunAnywayFor  []string
	hasOutput           bool   // Should show Output. Can be selected.
	viewportName        string // TODO: Implement Feature. Implement custom viewport setup. If left empty, will use the default viewport.
	viewportInputData   any    // Data that will be injected into the viewport for the output rendering.
	customSuccessStatus CustomTaskStatus
}

// NewTask creates a new task with the provided name.
//
// Optionally, you can provide the names of tasks for which this task,
// will run regardless of if they fail.
//
// This task's name and the names provided for other tasks, MUST match
// the column names.
func NewTask(name string, runAnyWayFor ...string) Task {
	t := Task{
		name:               name,
		status:             NotStarted,
		shouldRunAnywayFor: runAnyWayFor,
	}

	return t
}

type Column struct {
	Width int
	Name  string
}

type Styles struct {
	// NameHeader  lipgloss.Style
	Headers  lipgloss.Style
	Border   lipgloss.Style
	Cell     lipgloss.Style
	Selected lipgloss.Style
}

type KeyMap struct {
	// Summary
	LineUp          key.Binding
	LineDown        key.Binding
	PageDownSummary key.Binding
	PageUpSummary   key.Binding
	Quit            key.Binding
	Help            key.Binding

	// Output
	PageDown     key.Binding // Move a full page down in the output
	PageUp       key.Binding // Move a full page up in the output
	HalfPageUp   key.Binding // Move half a page up in the output
	HalfPageDown key.Binding // Move half a page down in the output
	Down         key.Binding // Move one line down in the output
	Up           key.Binding // Move one line up in the output
}

type RowKeyMap struct {
	ShowOutput key.Binding
	Left       key.Binding
	Right      key.Binding
}

// ShortHelp implements the KeyMap interface.
func (km KeyMap) ShortHelp() []key.Binding {
	return []key.Binding{km.LineUp, km.LineDown, km.Help, km.Quit}
}

// FullHelp implements the KeyMap interface.
func (km KeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{km.Quit, km.Help, km.LineUp, km.LineDown},
		{km.Down, km.Up, km.HalfPageDown, km.HalfPageUp, km.PageDownSummary},
		{km.PageUpSummary, km.PageUp, km.PageDown},
	}
}

// ShortHelp implements the KeyMap interface.
func (rm RowKeyMap) ShortHelp() []key.Binding {
	return []key.Binding{rm.Left, rm.Right, rm.ShowOutput}
}

// FullHelp implements the KeyMap interface.
func (rm RowKeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{{rm.Left, rm.Right, rm.ShowOutput}}
}

type Model struct {
	Help                     help.Model
	KeyMap                   KeyMap
	RowKeyMap                RowKeyMap
	showRowHelp              bool // showRowHelp is used when rendering the row help. It prevents rendering if there are no keys available for the row. It is set by SetActiveKeys()
	spinner                  spinner.Model
	taskColumns              []Column // columns contains all of the headers for the tasks. Does not include the "Name" column.
	prefixColumns            []Column // prefixColumns contains non-task columns that should be rendered to the left of the taskColumns.
	nameColumnWidth          int      // nameColumnWidth contains the width of the name column. The name column is treated specially.
	rows                     []Row
	selectedRow              int
	styles                   Styles
	showOutput               bool // showOutput indicates whether the output should be shown.
	termWidth                int  // termWidth contains the width of the terminal. This is used to dynamically size the output window.
	termHeight               int  // termHeight contains the height of the terminal. This is used to dynamically size the output window.
	pageSize                 int  // total rows per page
	pageCurrentStart         int  // the first row on the current page
	pageCurrentEnd           int  // the last row on the current page
	statusCounts             map[RowStatus]int
	defaultViewport          Viewport
	customActions            map[string]CustomAction // [CustomKeys].GetCustomActionName() should return a string that matches one of these keys.
	viewportWidthMultiplier  float64                 // width multiplier of the viewport. Viewport width will be set to this value * the terminal width. Default value: 0.6
	viewportHeightMultiplier float64                 // height multiplier of the viewport. Viewport height will be set to this value * the terminal height. Default value: 0.3
}

func NewModel(options ...Option) Model {
	m := Model{
		nameColumnWidth:          minNameLength,
		KeyMap:                   DefaultKeyMap(),
		RowKeyMap:                DefaultRowKeyMap(),
		Help:                     help.New(),
		pageSize:                 defaultPageSize,
		defaultViewport:          &DefaultViewport{},
		statusCounts:             make(map[RowStatus]int),
		viewportWidthMultiplier:  0.6,
		viewportHeightMultiplier: 0.3,
	}

	for _, opt := range options {
		opt(&m)
	}

	// Set the current page end index to the start index + page size unless there is only one page
	m.pageCurrentEnd = int(math.Min(float64(m.pageCurrentStart+m.pageSize)-1, float64(len(m.rows)-1)))

	// If a spinner wasn't set, set a default spinner.
	if m.spinner.ID() == 0 {
		m.spinner = spinner.New()
	}

	for k := range m.rows {
		m.rows[k].start()
		m.updateStatusCounts(m.rows[k].status, 0)
	}

	m.validateData()
	m.SetActiveKeys()

	return m
}

// validateData checks that each item has the same number of tasks as the number of headers.
//
// It panics if this is not the case. Panic will always be caught during testing.
func (m Model) validateData() {
	l := len(m.taskColumns) + len(m.prefixColumns)

	for _, row := range m.rows {
		if l != len(row.tasks)+len(row.prefixValues) {
			panic("the number of columns doesn't match the number of tasks + the number of other columns for all apps")
		}
	}
}

func (m Model) Init() tea.Cmd { return tea.Batch(m.spinner.Tick, tea.ClearScreen, tea.WindowSize()) }

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		// get the width and height of the terminal
		m.termWidth = msg.Width
		m.termHeight = msg.Height

		m.defaultViewport.SetDimensions(int(float64(m.termWidth)*m.viewportWidthMultiplier), int(float64(m.termHeight)*m.viewportHeightMultiplier))

	case tea.KeyMsg:
		var shouldSetActiveKeys, shouldUpdateOutput bool

		switch {
		case key.Matches(msg, m.RowKeyMap.Left):
			// While output is showing, move left
			m.rows[m.selectedRow].moveLeft()
			shouldSetActiveKeys, shouldUpdateOutput = true, true

		case key.Matches(msg, m.RowKeyMap.Right):
			// While output is showing, move right
			m.rows[m.selectedRow].moveRight()
			shouldSetActiveKeys, shouldUpdateOutput = true, true

		case key.Matches(msg, m.RowKeyMap.ShowOutput):
			// show output
			m.showOutput = !m.showOutput
			shouldSetActiveKeys, shouldUpdateOutput = true, m.showOutput

		case key.Matches(msg, m.KeyMap.LineDown):
			// Line down in the summary
			m.LineDown()
			m.rows[m.selectedRow].setSelected()

			shouldSetActiveKeys, shouldUpdateOutput = true, m.showOutput

		case key.Matches(msg, m.KeyMap.LineUp):
			// Line up in the summary
			m.LineUp()
			m.rows[m.selectedRow].setSelected()
			shouldSetActiveKeys, shouldUpdateOutput = true, m.showOutput

		case key.Matches(msg, m.KeyMap.PageDownSummary):
			// Page down in the summary
			m.PageDown()
			m.rows[m.selectedRow].setSelected()
			shouldSetActiveKeys, shouldUpdateOutput = true, m.showOutput

		case key.Matches(msg, m.KeyMap.PageUpSummary):
			// Page up in the summary
			m.PageUp()
			m.rows[m.selectedRow].setSelected()
			shouldSetActiveKeys, shouldUpdateOutput = true, m.showOutput

		case key.Matches(msg, m.KeyMap.Quit):
			// Quit application
			return m, tea.Quit

		case key.Matches(msg, m.KeyMap.Help):
			// Show additional help
			m.Help.ShowAll = !m.Help.ShowAll

		case key.Matches(msg, m.KeyMap.Down):
			// While output is shown, scroll down
			cmds = append(cmds, m.defaultViewport.LineDown(1))

		case key.Matches(msg, m.KeyMap.Up):
			// While output is shown, scroll up
			cmds = append(cmds, m.defaultViewport.LineUp(1))

		case key.Matches(msg, m.KeyMap.PageDown):
			// While output is shown, scroll page down
			cmds = append(cmds, m.defaultViewport.ViewDown())

		case key.Matches(msg, m.KeyMap.PageUp):
			// While output is shown, scroll page up
			cmds = append(cmds, m.defaultViewport.ViewUp())

		case key.Matches(msg, m.KeyMap.HalfPageDown):
			// While output is shown, scroll half page down
			cmds = append(cmds, m.defaultViewport.HalfViewDown())

		case key.Matches(msg, m.KeyMap.HalfPageUp):
			// While output is shown, scroll half page up
			cmds = append(cmds, m.defaultViewport.HalfViewUp())
		}

		if shouldSetActiveKeys {
			m.SetActiveKeys()
		}

		if shouldUpdateOutput {
			if m.rows[m.selectedRow].hasOutputToDisplay() {
				// If a row has output to display, update the viewport
				cmds = append(cmds, m.setOutput())
			} else {
				// Otherwise switch off showOutput
				m.showOutput = false
			}
		}

	case TaskResultMsg:
		prev, cur := m.rows[msg.Index].update(msg)
		m.updateStatusCounts(cur, prev)

		if msg.Index == m.selectedRow {
			m.rows[msg.Index].setSelected()

			// Reload available keys on the currently selected row.
			m.SetActiveKeys()
		}
	}

	m.spinner, cmd = m.spinner.Update(msg)
	cmds = append(cmds, cmd)
	m.defaultViewport, cmd = m.defaultViewport.Update(msg)
	cmds = append(cmds, cmd)
	return m, tea.Batch(cmds...)
}

// updateStatusCounts updates the aggregated count of the row statuses.
//
// The status matching addStatus, will be increased by 1 and the status matching removeStatus will be decreased by 1.
func (m *Model) updateStatusCounts(addStatus RowStatus, removeStatus RowStatus) {
	if addStatus != 0 {
		m.statusCounts[addStatus] += 1
	}

	if removeStatus != 0 {
		if val, ok := m.statusCounts[removeStatus]; ok {
			if val-1 > 0 {
				m.statusCounts[removeStatus]--
			} else {
				delete(m.statusCounts, removeStatus)
			}
		}
	}
}

// LineDown goes to the previous line in the reportcard summary table.
func (m *Model) LineUp() {
	if m.selectedRow-1 >= 0 {
		m.selectedRow -= 1
	}

	if m.selectedRow < m.pageCurrentStart {
		_, _, m.pageCurrentStart, m.pageCurrentEnd = GetPaginationDetails(len(m.rows), m.pageSize, m.selectedRow)
	}
}

// LineDown goes to the next line in the reportcard summary table.
func (m *Model) LineDown() {
	if m.selectedRow+1 < len(m.rows) {
		m.selectedRow += 1
	}

	if m.selectedRow > m.pageCurrentEnd {
		_, _, m.pageCurrentStart, m.pageCurrentEnd = GetPaginationDetails(len(m.rows), m.pageSize, m.selectedRow)
	}
}

// PageUp goes to the previous page in the reportcard summary table.
func (m *Model) PageUp() {
	if m.pageCurrentStart-m.pageSize >= 0 {
		m.selectedRow = m.pageCurrentStart - m.pageSize
		_, _, m.pageCurrentStart, m.pageCurrentEnd = GetPaginationDetails(len(m.rows), m.pageSize, m.selectedRow)
	}
}

// PageDown goes to the next page in the reportcard summary table.
func (m *Model) PageDown() {
	if m.pageCurrentEnd+1 < len(m.rows) {
		m.selectedRow = m.pageCurrentEnd + 1
		_, _, m.pageCurrentStart, m.pageCurrentEnd = GetPaginationDetails(len(m.rows), m.pageSize, m.selectedRow)
	}
}

// SetActiveKeys enables/disables global keys and keys for the selected row.
// TODO: Implement Feature. Add custom keys/actions.
func (m *Model) SetActiveKeys() {
	// Global keys
	m.KeyMap.LineDown.SetEnabled(m.selectedRow < len(m.rows)-1)
	m.KeyMap.LineUp.SetEnabled(m.selectedRow > 0)
	m.KeyMap.Down.SetEnabled(m.showOutput)
	m.KeyMap.Up.SetEnabled(m.showOutput)
	m.KeyMap.PageDown.SetEnabled(m.showOutput)
	m.KeyMap.PageUp.SetEnabled(m.showOutput)
	m.KeyMap.HalfPageDown.SetEnabled(m.showOutput)
	m.KeyMap.HalfPageUp.SetEnabled(m.showOutput)

	// Keys for the selected row
	left := m.rows[m.selectedRow].shouldShowLeft() && m.showOutput
	right := m.rows[m.selectedRow].shouldShowRight() && m.showOutput
	showOutput := m.rows[m.selectedRow].hasOutputToDisplay()

	m.RowKeyMap.Left.SetEnabled(left)
	m.RowKeyMap.Right.SetEnabled(right)
	m.RowKeyMap.ShowOutput.SetEnabled(showOutput)

	m.showRowHelp = left || right || showOutput
}

// setOutput updates relevant [Viewport] with the task's output.
func (m *Model) setOutput() tea.Cmd {
	if r := m.rows[m.selectedRow]; r.hasOutputToDisplay() && m.showOutput {
		viewportName, content := r.getOutput()

		switch viewportName {
		default:
			if !m.defaultViewport.HasBeenInitialized() {
				return m.defaultViewport.Init(int(float64(m.termWidth)*m.viewportWidthMultiplier), int(float64(m.termHeight)*m.viewportHeightMultiplier), content)
			}
			m.defaultViewport.SetContent(content)
		}
	}

	return nil
}

func (m Model) View() string {
	// length of rows includes the header
	rows := make([]string, 0, len(m.rows)+1)
	rows = append(rows, m.headerView())

	for i := m.pageCurrentStart; i <= m.pageCurrentEnd; i++ {
		rows = append(rows, m.renderRow(i))
	}

	summaryTableRendered := lipgloss.JoinVertical(lipgloss.Top, rows...)
	metaDataRendered := m.renderMetaData(lipgloss.Width(summaryTableRendered))

	summary := m.styles.Border.Render(lipgloss.JoinVertical(lipgloss.Left, summaryTableRendered, metaDataRendered))

	// If there is more than one page, render the "scroll bar"
	if len(m.rows) > m.pageSize {
		summary = lipgloss.JoinHorizontal(
			lipgloss.Center,
			summary,
			renderPageScrollBar(m.pageCurrentStart, m.pageCurrentEnd, len(m.rows), m.Help.Styles.FullDesc, m.Help.Styles.FullKey, m.KeyMap.PageUpSummary.Keys()[0], m.KeyMap.PageDownSummary.Keys()[0]),
		)
	}

	// I am doing below because lipgloss.JoinVertical() adds all strings to a new line, even if they are empty.
	// I don't want there to be an empty line between the table and the help when output is not rendered.
	if m.showOutput {
		output := m.renderOutput()
		return lipgloss.JoinVertical(lipgloss.Left, summary, output, m.renderHelp())
	} else {
		return lipgloss.JoinVertical(lipgloss.Left, summary, m.renderHelp())
	}
}

func (m Model) renderHelp() string {
	var h string
	if m.showRowHelp {
		h += m.styles.Selected.Inline(true).Render("[") + " " + m.Help.View(m.RowKeyMap) + " " + m.styles.Selected.Inline(true).Render("]") + "\n"
	}
	h += m.Help.View(m.KeyMap)

	return h
}

func (m Model) renderOutputScrollBar(arrowStyle, keyStyle lipgloss.Style) string {
	var s string
	if m.defaultViewport.AtTop() {
		s += arrowStyle.Render("┬")
	} else {
		s += arrowStyle.Render("↟") + " " + keyStyle.Render(strings.Join(m.KeyMap.PageUp.Keys(), ",")) + "\n"
		s += arrowStyle.Render("↑") + " " + keyStyle.Render(strings.Join(m.KeyMap.Up.Keys(), ","))
	}

	s += "\n"

	if m.defaultViewport.AtBottom() {
		s += arrowStyle.Render("┴")
	} else {
		s += arrowStyle.Render("↓") + " " + keyStyle.Render(strings.Join(m.KeyMap.Down.Keys(), ",")) + "\n"
		s += arrowStyle.Render("↡") + " " + keyStyle.Render(strings.Join(m.KeyMap.PageDown.Keys(), ","))
	}

	return s
}

func (m Model) renderOutput() string {
	var output string

	if m.showOutput {
		output = m.styles.Border.Render(fmt.Sprintf("%s\n\n", lipgloss.NewStyle().Bold(true).Render("Output")) + m.defaultViewport.View())

		if m.defaultViewport.ShouldShowScrollBar() {
			output = lipgloss.JoinHorizontal(
				lipgloss.Center,
				output,
				m.renderOutputScrollBar(m.Help.Styles.FullDesc, m.Help.Styles.FullKey),
			)
		}
	}

	return output
}

func renderPageScrollBar(pageStart, pageEnd, totalElements int, arrowStyle, keyStyle lipgloss.Style, pageUpKey, pageDownKey string) string {
	var s string

	if pageStart != 0 {
		s += arrowStyle.Render("↟") + " " + keyStyle.Render(pageUpKey)

	} else {
		s += arrowStyle.Render("┬")
	}

	s += "\n"

	if pageEnd != totalElements-1 {
		s += arrowStyle.Render("↡") + " " + keyStyle.Render(pageDownKey)
	} else {
		s += arrowStyle.Render("┴")
	}

	return s
}

// headerView renders the content for the summary table's headers.
func (m Model) headerView() string {
	// s is set to the length of: white space in front + name column + task columns + prefixColumns
	s := make([]string, 0, len(m.taskColumns)+len(m.prefixColumns)+2)
	s = append(s, " ")

	columns := slices.Concat([]Column{{Name: "Name", Width: m.nameColumnWidth}}, m.prefixColumns, m.taskColumns)

	for k, col := range columns {
		style := lipgloss.NewStyle().Width(col.Width).MaxWidth(col.Width).Inline(true)

		// (columns contains the name, prefix and task columns. The selected task index is only for the task list. Below normalizes it.)
		if m.showOutput && k-1-len(m.prefixColumns) == m.rows[m.selectedRow].getSelectedIndex() {
			// Highlight the column header of the selected task.
			style = m.styles.Selected.Inherit(style)
		}

		renderedCell := m.styles.Headers.Render(style.Render(col.Name))
		s = append(s, renderedCell)
	}

	return lipgloss.JoinHorizontal(lipgloss.Top, s...)
}

// renderRow renders the content for a row with index i.
func (m Model) renderRow(rowIndex int) string {
	// s is set to the length of: white space in front + name column + task columns + prefixColumns
	s := make([]string, 1, len(m.taskColumns)+len(m.prefixColumns)+2)

	columns := slices.Concat([]Column{{Width: m.nameColumnWidth}}, m.prefixColumns, m.taskColumns)

	var content string

	for colIndex, col := range columns {
		style := lipgloss.NewStyle().Width(col.Width).MaxWidth(col.Width).Inline(true).Align(lipgloss.Left)
		flavourStyle := lipgloss.NewStyle()
		var useFlavourStyle bool

		if colIndex == 0 {
			// Name column
			content = m.rows[rowIndex].name
		}

		if colIndex > 0 && colIndex < len(m.prefixColumns)+1 {
			// Prefix columns
			content = m.rows[rowIndex].prefixValues[colIndex-1]
		}

		if colIndex >= len(m.prefixColumns)+1 {
			// Task columns
			taskStatusSummary := m.rows[rowIndex].GetTaskStatusSummary(colIndex - len(m.prefixColumns) - 1)

			if taskStatusSummary.status == InProgress {
				// If the status is InProgress, use the Model.spinner
				style = style.AlignHorizontal(lipgloss.Center)
				content = m.spinner.View()

			} else if taskStatusSummary.useCustom {
				// If the status is Success and uses a custom value
				content = taskStatusSummary.content
				style = style.AlignHorizontal(taskStatusSummary.alignment)

				useFlavourStyle = true
				flavourStyle = flavourStyle.Foreground(taskStatusSummary.colour)

			} else {
				// If the status does not use a custom value
				content = taskStatusSummary.content
				style = style.AlignHorizontal(lipgloss.Center)

				useFlavourStyle = true
				flavourStyle = flavourStyle.Foreground(taskStatusSummary.colour)
			}

			if useFlavourStyle {
				content = flavourStyle.Render(taskStatusSummary.content)
			}

			if rowIndex == m.selectedRow && taskStatusSummary.isSelected && m.showOutput {
				// Row is selected, task is selected and output is switched on by the user.
				content = m.styles.Selected.Inline(true).Render("◜") + content + m.styles.Selected.Inline(true).Render("◞")
			}
		}

		renderedCell := m.styles.Cell.Render(style.Render(content))

		s = append(s, renderedCell)
	}

	if rowIndex == m.selectedRow {
		s[0] = m.styles.Selected.Inline(true).Render("❖")
	} else {
		s[0] = " "
	}

	return lipgloss.JoinHorizontal(lipgloss.Top, s...)
}

// renderMetaData returns the rendered page counts and total counts at the bottom of the reportcard.
func (m Model) renderMetaData(tableWidth int) string {
	var s string
	pageCountsRendered := m.renderPageCounts()
	totalCountsRendered := m.renderTotalCounts()
	// Calculate the the space between the page count (left-aligned) and the total count (right-aligned)
	spaceBetween := tableWidth - lipgloss.Width(totalCountsRendered) - lipgloss.Width(pageCountsRendered)

	// If the spaceBetween the left-aligned and the right-aligned content is too little, display them underneath each other.
	// (5 is an arbitrary number in this case)
	if spaceBetween <= 5 {
		s = pageCountsRendered + "\n" + lipgloss.NewStyle().PaddingLeft(m.styles.Headers.GetPaddingLeft()+1).Render(totalCountsRendered)
	} else {
		s = pageCountsRendered + strings.Repeat(" ", spaceBetween) + lipgloss.NewStyle().PaddingRight(m.styles.Headers.GetPaddingRight()).Render(totalCountsRendered)
	}

	return s
}

// renderTotalCounts returns the rendered aggregated total counts. It is only called by renderMetaData().
func (m Model) renderTotalCounts() string {
	ls := make([]string, 0, len(m.statusCounts))

	for _, status := range []RowStatus{RowUserPrompt, RowWarning, RowNotStarted, RowStarted, RowLoading, RowSuccess, RowFailure} {
		if count, ok := m.statusCounts[status]; ok {
			ls = append(ls, fmt.Sprintf("%s %d", GetRowStatusSymbol(status), count))
		}
	}

	s := strings.Join(ls, " ")

	return lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{
		Light: "#909090",
		Dark:  "#626262",
	}).Render(fmt.Sprintf("total: (%s) / %d", s, len(m.rows)))
}

// renderPageCounts returns the rendered page meta data. It is only called by renderMetaData().
func (m Model) renderPageCounts() string {
	numPages, currPage, _, _ := GetPaginationDetails(len(m.rows), m.pageSize, m.pageCurrentStart)

	return lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{
		Light: "#909090",
		Dark:  "#626262",
		// PaddingLeft(): +1 for the space/selection indicator in front of the row
	}).PaddingLeft(m.styles.Headers.GetPaddingLeft() + 1).Render(fmt.Sprintf("page: %d of %d", currPage+1, numPages))
}

func GetPaginationDetails(totalElements, elementsPerPage, currentElementIndex int) (numPages int, currentPage int, pageStartIndex int, pageEndIndex int) {
	numPages = int(math.Ceil(float64(totalElements) / float64(elementsPerPage)))
	pageStartIndex = (currentElementIndex / elementsPerPage) * elementsPerPage
	pageEndIndex = int(math.Min(float64(pageStartIndex+elementsPerPage-1), float64(totalElements-1)))
	currentPage = int(math.Ceil(float64(pageStartIndex) / float64(elementsPerPage)))
	return
}

func GetTaskStatusSymbols(status TaskStatus) (string, lipgloss.Color) {
	switch status {
	case Warning:
		return "⚠", lipgloss.Color("#FFA500")

	case Success:
		return "✓", lipgloss.Color("42")

	case Failure:
		return "✗", lipgloss.Color("9")

	case Skip:
		return "-", lipgloss.Color("#767676")

	case NotStarted:
		return "!", lipgloss.Color("#767676")

	default:
		return "Err", lipgloss.Color("42")
	}
}

// WithColumns sets the report card's columns (headers).
func WithTasks(cols []Column) Option {
	return func(m *Model) {
		if cols != nil {
			m.taskColumns = cols
		} else {
			m.taskColumns = make([]Column, 0)
		}
	}
}

// WithPrefixColumns sets the report card's prefix columns (headers). Prefix columns are non-task columns and will be rendered
// to the left of the task columns.
func WithPrefixColumns(cols []Column) Option {
	return func(m *Model) {
		if cols != nil {
			m.prefixColumns = cols
		} else {
			m.prefixColumns = make([]Column, 0)
		}
	}
}

// WithData sets the report card's rows.
func WithData(rows ...Row) Option {
	return func(m *Model) {
		if rows != nil {
			m.rows = rows
		} else {
			m.rows = make([]Row, 0)
		}

		// The name column is sized dynamically based on the longest name.
		for _, row := range m.rows {
			if l := lipgloss.Width(row.name); l > m.nameColumnWidth {
				m.nameColumnWidth = l
			}
		}

		// The min width for the column is minNameLength
		m.nameColumnWidth = int(math.Max(float64(m.nameColumnWidth), float64(minNameLength)))

	}
}

// WithStyles sets the report card's styles.
func WithStyles(styles Styles) Option {
	return func(m *Model) {
		m.styles = styles
	}
}

// WithSpinner sets the spinner options for the report card.
func WithSpinner(opts ...spinner.Option) Option {
	return func(m *Model) {
		m.spinner = spinner.New(opts...)
	}
}

// WithKeyMap sets the KeyMap for the report card.
func WithKeyMap(km KeyMap) Option {
	return func(m *Model) {
		m.KeyMap = km
	}
}

// Set the page size of the report card.
func WithPageSize(pageSize int) Option {
	return func(m *Model) {
		m.pageSize = int(math.Max(float64(pageSize), 1))
	}
}

func GetRowStatusSymbol(rowStatus RowStatus) string {
	switch rowStatus {
	case RowFailure:
		return "✗"
	case RowStarted, RowLoading:
		return "⣯"
	case RowNotStarted:
		return "!"
	case RowSuccess:
		return "✓"
	case RowUserPrompt:
		return "?"
	case RowWarning:
		return "⚠"
	default:
		return ""
	}
}

func WithRowKeyMap(rowKeyMap RowKeyMap) Option {
	return func(m *Model) {
		m.RowKeyMap = rowKeyMap
	}
}

func WithViewportDimensions(widthModifier, heightModifier float64) Option {
	if widthModifier > 1 || widthModifier < 0.1 {
		widthModifier = 0.6
	}

	if heightModifier > 1 || heightModifier < 0.1 {
		heightModifier = 0.3
	}

	return func(m *Model) {
		m.viewportHeightMultiplier = heightModifier
		m.viewportWidthMultiplier = widthModifier
	}
}

func WithHelp(help help.Model) Option {
	return func(m *Model) {
		m.Help = help
	}
}

// DefaultKeyMap returns a default set of keybindings.
func DefaultKeyMap() KeyMap {
	return KeyMap{
		Quit: key.NewBinding(
			key.WithKeys("ctrl+c"),
			key.WithHelp("ctrl+c", "quit"),
		),
		Help: key.NewBinding(
			key.WithKeys("?"),
			key.WithHelp("?", "toggle help"),
		),
		LineUp: key.NewBinding(
			key.WithKeys("up"),
			key.WithHelp("↑", "up"),
		),
		LineDown: key.NewBinding(
			key.WithKeys("down"),
			key.WithHelp("↓", "down"),
		),
		PageDownSummary: key.NewBinding(
			key.WithKeys("g"),
			key.WithHelp("g", "page down"),
		),
		PageUpSummary: key.NewBinding(
			key.WithKeys("t"),
			key.WithHelp("t", "page down"),
		),
		PageDown: key.NewBinding(
			key.WithKeys("pgdown", "f"),
			key.WithHelp("f/pgdn", "page down"),
			key.WithDisabled(),
		),
		PageUp: key.NewBinding(
			key.WithKeys("pgup", "b"),
			key.WithHelp("b/pgup", "page up"),
			key.WithDisabled(),
		),
		HalfPageUp: key.NewBinding(
			key.WithKeys("u", "ctrl+u"),
			key.WithHelp("u", "½ page up"),
			key.WithDisabled(),
		),
		HalfPageDown: key.NewBinding(
			key.WithKeys("d", "ctrl+d"),
			key.WithHelp("d", "½ page down"),
			key.WithDisabled(),
		),
		Up: key.NewBinding(
			key.WithKeys("k"),
			key.WithHelp("k", "scroll up"),
			key.WithDisabled(),
		),
		Down: key.NewBinding(
			key.WithKeys("j"),
			key.WithHelp("j", "scroll down"),
			key.WithDisabled(),
		),
	}
}

// DefaultRowKeyMap returns a default set of keybindings for the row.
func DefaultRowKeyMap() RowKeyMap {
	return RowKeyMap{
		Left: key.NewBinding(
			key.WithKeys("left"),
			key.WithHelp("←", "left"),
			key.WithDisabled(),
		),
		Right: key.NewBinding(
			key.WithKeys("right"),
			key.WithHelp("→", "right"),
			key.WithDisabled(),
		),
		ShowOutput: key.NewBinding(
			key.WithKeys("s"),
			key.WithHelp("s", "toggle output"),
			key.WithDisabled(),
		),
	}
}
