package reportcard

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type TaskStatus int

const (
	Success TaskStatus = iota
	InProgress
	Failure
	Skip
	NotStarted
)

const (
	// Minimum width of the name column. (the name column is sized dynamically based on the longest name)
	minNameLength = 4

	// Default page size. Will be used if no page size is provided.
	defaultPageSize = 6
)

type TaskResultMsg struct {
	Status              TaskStatus
	Index               int              // Index is the index of the item in [Model].rows
	taskIndex           int              // Gets set internally after a failure. This is used to match failures to specific tasks on the frontend.
	Output              string           // Should be set on failure. Will be displayed to the end user.
	IsFatal             bool             // Skip all following tasks
	CustomSuccessStatus CustomTaskStatus //  CustomTaskStatus contains options for replacing the normal success symbol.
}

type CustomTaskStatus struct {
	Message          string // If Message is set, the alignment of the text in the column will be left-aligned.
	ForegroundColour string
}

type Task struct {
	Status TaskStatus
	// ShouldRunAnywayFor allows a task to "run" on the reportcard regardless of if a previous task
	// failed fatally. ShouldRunAnywayFor is a map[int]bool where the keys are task indexes and the
	// values are bools indicating whether the task should run for said task.
	ShouldRunAnywayFor  map[int]bool
	customSuccessStatus CustomTaskStatus
}

type Row struct {
	Name         string
	Tasks        []Task
	PrefixValues []string
	FinalStatus  TaskStatus
}

type Column struct {
	Width int
	Name  string
}

type Styles struct {
	NameHeader  lipgloss.Style
	TaskHeaders lipgloss.Style
	Border      lipgloss.Style
	Cell        lipgloss.Style
	Selected    lipgloss.Style
}

type KeyMap struct {
	// Summary
	LineUp          key.Binding
	LineDown        key.Binding
	ColLeft         key.Binding
	ColRight        key.Binding
	PageDownSummary key.Binding
	PageUpSummary   key.Binding
	ShowOutput      key.Binding
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

// report card assumes that tasks are completed sequentially and it assumes that once a task is done, it is done.
type Model struct {
	Help            help.Model
	KeyMap          KeyMap
	spinner         spinner.Model
	taskColumns     []Column // columns contains all of the headers for the tasks. Does not include the "Name" column.
	prefixColumns   []Column // prefixColumns contains non-task columns that should be rendered to the left of the taskColumns.
	nameColumnWidth int      // nameColumnWidth contains the width of the name column. The name column is treated specially.
	rows            []Row
	// output          output
	// activeTasks is a map[int]int where the key is an index for Model.rows and the value is the index for Model.rows[n].Tasks[].
	// activeTasks stores which tasks are currently in progress for all of the items. Entries are deleted once their tasks are finished.
	activeTasks           map[int]int
	styles                Styles
	canShowOutput         bool // an error has occurred for one of the tasks and there is output available that can be shown to the user.
	showOutput            bool // showOutput indicates whether the output should be shown.
	selector              selector[TaskResultMsg]
	termWidth             int // termWidth contains the width of the terminal. This is used to dynamically size the output window.
	termHeight            int // termHeight contains the height of the terminal. This is used to dynamically size the output window.
	notFirstCompletedTask bool

	pageSize         int // total rows per page
	pageCurrentStart int // the first row on the current page
	pageCurrentEnd   int // the last row on the current page

	failureCount    int
	successCount    int
	inProgressCount int

	defaultViewport Viewport
}

// Option is used to set options in New. For example:
//
//	table := New(WithColumns([]Column{{Title: "ID", Width: 10}}))
type Option func(*Model)

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
func WithData(rows []Row) Option {
	return func(m *Model) {
		if rows != nil {
			m.rows = rows
		} else {
			m.rows = make([]Row, 0)
		}

		// The name column is sized dynamically based on the longest name.
		for _, row := range m.rows {
			if l := lipgloss.Width(row.Name); l > m.nameColumnWidth {
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

func NewModel(options ...Option) Model {
	m := Model{
		nameColumnWidth: minNameLength,
		activeTasks:     make(map[int]int),
		KeyMap:          DefaultKeyMap(),
		Help:            help.New(),
		pageSize:        defaultPageSize,
		defaultViewport: &DefaultViewport{},
	}

	for _, opt := range options {
		opt(&m)
	}

	// Set the current page end index to the start index + page size unless there is only one page
	m.pageCurrentEnd = int(math.Min(float64(m.pageCurrentStart+m.pageSize)-1, float64(len(m.rows)-1)))

	m.selector = newSelector[TaskResultMsg](len(m.taskColumns), len(m.rows))

	// If a spinner wasn't set, set a default spinner.
	if m.spinner.ID() == 0 {
		m.spinner = spinner.New()
	}

	m.inProgressCount = len(m.rows)

	for k := range m.rows {
		m.handleRemainingTasks(k, 0, false)
	}

	m.validateData()

	return m
}

// validateData checks that each item has the same number of tasks as the number of headers.
//
// It panics if this is not the case. Panic will always be caught during testing.
func (m Model) validateData() {
	l := len(m.taskColumns) + len(m.prefixColumns)

	for _, row := range m.rows {
		if l != len(row.Tasks)+len(row.PrefixValues) {
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

		m.defaultViewport.SetDimensions(int(float64(m.termWidth)*0.6), int(float64(m.termHeight)*0.3))

	case tea.KeyMsg:
		switch {
		case key.Matches(msg, m.KeyMap.Quit):
			return m, tea.Quit

		case key.Matches(msg, m.KeyMap.Help):
			m.Help.ShowAll = !m.Help.ShowAll

		case key.Matches(msg, m.KeyMap.ColLeft):
			m.MoveOne(left)

		case key.Matches(msg, m.KeyMap.ColRight):
			m.MoveOne(right)

		case key.Matches(msg, m.KeyMap.LineDown):
			m.MoveOne(down)

		case key.Matches(msg, m.KeyMap.LineUp):
			m.MoveOne(up)

		case key.Matches(msg, m.KeyMap.PageDownSummary):
			m.PageDown()

		case key.Matches(msg, m.KeyMap.PageUpSummary):
			m.PageUp()

		case key.Matches(msg, m.KeyMap.ShowOutput):
			// If there is output to show, show/hide the output.
			m.showOutput = !m.showOutput

			if m.showOutput {
				item, row, _ := m.selector.GetSelected()

				if row < m.pageCurrentStart || row > m.pageCurrentEnd {
					item, _, _, _ = m.selector.SetSelectedInRange(m.pageCurrentStart, m.pageCurrentEnd+1)
				}

				if !m.defaultViewport.HasBeenInitialized() {
					// Initialize the viewport for the first time.
					cmds = append(cmds, m.defaultViewport.Init(int(float64(m.termWidth)*0.6), int(float64(m.termHeight)*0.3), item.Output))
				} else if m.selector.HasChanged() {
					// Update the content if the viewport has already been initialized and if the selector has moved.
					// i.e. don't reload the viewport if nothing has changed.
					m.defaultViewport.SetContent(item.Output)
				}
			}

		case key.Matches(msg, m.KeyMap.PageDown):
			cmds = append(cmds, m.defaultViewport.ViewDown())

		case key.Matches(msg, m.KeyMap.PageUp):
			cmds = append(cmds, m.defaultViewport.ViewUp())

		case key.Matches(msg, m.KeyMap.HalfPageDown):
			cmds = append(cmds, m.defaultViewport.HalfViewDown())

		case key.Matches(msg, m.KeyMap.HalfPageUp):
			cmds = append(cmds, m.defaultViewport.HalfViewUp())

		case key.Matches(msg, m.KeyMap.Down):
			cmds = append(cmds, m.defaultViewport.LineDown(1))

		case key.Matches(msg, m.KeyMap.Up):
			cmds = append(cmds, m.defaultViewport.LineUp(1))

		}
	case TaskResultMsg:
		// if the incoming msg is for a task that is still active, handle it, otherwise
		// ignore it. This handles the scenario where the sender continues to send msgs
		// after all tasks should be in a final status.
		if taskIndex, ok := m.activeTasks[msg.Index]; ok {
			if !m.notFirstCompletedTask {
				// for that first task that completes, enable keys and set the output content.
				m.setCanShowOutput()
				m.notFirstCompletedTask = true
			}
			msg.taskIndex = taskIndex
			m.selector.AddSelectable(msg, msg.Index, msg.taskIndex)

			m.rows[msg.Index].Tasks[m.activeTasks[msg.Index]].customSuccessStatus = msg.CustomSuccessStatus

			switch msg.Status {
			case Success:
				m.rows[msg.Index].Tasks[m.activeTasks[msg.Index]].Status = Success
				m.handleRemainingTasks(msg.Index, m.activeTasks[msg.Index], false)

			case Failure:
				m.rows[msg.Index].Tasks[m.activeTasks[msg.Index]].Status = Failure
				m.handleRemainingTasks(msg.Index, m.activeTasks[msg.Index], msg.IsFatal)
			}
		}
	}

	m.updateAvailableKeys()

	m.spinner, cmd = m.spinner.Update(msg)
	cmds = append(cmds, cmd)
	m.defaultViewport, cmd = m.defaultViewport.Update(msg)
	cmds = append(cmds, cmd)
	return m, tea.Batch(cmds...)
}

func (m *Model) MoveOne(direction direction) {
	// If there is output to show, move down on the table.
	item, row, _, didMove := m.selector.MoveCursor(direction, 1)
	if didMove {
		// Only update the content if the cursor actually moved.
		// m.output.SetContent(item.Output, int(float64(m.termWidth)*0.6))
		m.defaultViewport.SetContent(item.Output)

		_, _, pageStart, pageEnd := GetPaginationDetails(len(m.rows), m.pageSize, row)
		m.pageCurrentStart = pageStart
		m.pageCurrentEnd = pageEnd
	}
}

func (m *Model) PageDown() {
	if m.pageCurrentEnd == len(m.rows)-1 {
		return
	}

	_, _, pageStart, pageEnd := GetPaginationDetails(len(m.rows), m.pageSize, m.pageCurrentEnd+1)

	m.pageCurrentEnd = pageEnd
	m.pageCurrentStart = pageStart

	if m.showOutput {
		item := m.selector.SetSelected(m.pageCurrentStart, m.selector.selectedItemColumn)
		m.defaultViewport.SetContent(item.Output)
	}
}

func (m *Model) PageUp() {
	if m.pageCurrentStart == 0 {
		return
	}

	_, _, pageStart, pageEnd := GetPaginationDetails(len(m.rows), m.pageSize, m.pageCurrentStart-1)

	m.pageCurrentStart = pageStart
	m.pageCurrentEnd = pageEnd

	if m.showOutput {
		item := m.selector.SetSelected(m.pageCurrentStart, m.selector.selectedItemColumn)
		m.defaultViewport.SetContent(item.Output)
	}
}

// setCanShowOutput allows the user to toggle on/of the error/standard output for the tasks.
func (m *Model) setCanShowOutput() {
	m.canShowOutput = true
	m.KeyMap.ShowOutput.SetEnabled(true)
}

func (m *Model) updateAvailableKeys() {
	_, _, found := m.selector.GetSelectedInRange(m.pageCurrentStart, m.pageCurrentEnd+1)

	m.KeyMap.ShowOutput.SetEnabled(found)

	if m.showOutput {
		m.KeyMap.LineDown.SetEnabled(true)
		m.KeyMap.LineUp.SetEnabled(true)
		m.KeyMap.ColLeft.SetEnabled(true)
		m.KeyMap.ColRight.SetEnabled(true)
		m.KeyMap.Down.SetEnabled(true)
		m.KeyMap.Up.SetEnabled(true)
	} else {
		m.KeyMap.LineDown.SetEnabled(false)
		m.KeyMap.LineUp.SetEnabled(false)
		m.KeyMap.ColLeft.SetEnabled(false)
		m.KeyMap.ColRight.SetEnabled(false)
		m.KeyMap.Down.SetEnabled(false)
		m.KeyMap.Up.SetEnabled(false)
	}
}

// handleRemainingTasks is executed after a task succeeds or fails. It loops through the list
// of tasks after the current task, and sets their status based on whether it was a fatal failure
// and whether the task is set to run regardless of a fatal failure.
//
// If there are no more tasks that can be moved to In Progress, it means that the item is effectively
// done in which case the method deletes the index from m.activeTasks.
//
// handleRemainingTasks has below parameters:
//   - i int 			(id/index of the item)
//   - taskIndex int 	(index of the task from which to check onwards)
//   - isFatal bool 	(bool indicating if the task failed fatally)
func (m *Model) handleRemainingTasks(i int, taskIndex int, isFatal bool) {
	var rowNotDone bool

	if isFatal {
		m.rows[i].FinalStatus = Failure
	}

	for k := taskIndex; k < len(m.rows[i].Tasks); k++ {
		if m.rows[i].Tasks[k].Status == NotStarted {

			if isFatal {
				if m.rows[i].Tasks[k].ShouldRunAnywayFor[taskIndex] {
					// If the original task failed fatally, but the current
					// task is allowed to run anyway, change its status to
					// InProgress
					m.rows[i].Tasks[k].Status = InProgress
					rowNotDone = true
					m.activeTasks[i] = k

				} else {
					m.rows[i].Tasks[k].Status = Skip
				}

			} else {
				m.rows[i].Tasks[k].Status = InProgress
				rowNotDone = true
				m.activeTasks[i] = k
				break
			}
		}
	}

	if !rowNotDone {
		// Row is done, there are no more tasks to run.

		// Update the total counts for the different statuses.
		switch m.rows[i].Tasks[taskIndex].Status {
		case Failure:
			m.failureCount++
			m.inProgressCount--
		case Success:
			if m.rows[i].FinalStatus != Failure {
				// Some tasks can run after a previous task fails fatally.
				// This makes sure that the final result for the row is shown
				// correctly.
				m.successCount++
			} else {
				m.failureCount++
			}

			m.inProgressCount--
		}

		delete(m.activeTasks, i)
	}
}

func (m Model) View() string {
	// length of rows includes the header
	rows := make([]string, 0, len(m.rows)+1)
	rows = append(rows, m.headerView())
	for i := m.pageCurrentStart; i <= m.pageCurrentEnd; i++ {
		rows = append(rows, m.renderRow(i))
	}

	summaryTableRendered := lipgloss.JoinVertical(lipgloss.Top, rows...)

	pageCountsRendered := m.renderPageCounts()
	totalCountsRendered := m.renderTotalCounts()

	// Calculate the the space between the page count (left-aligned) and the total count (right-aligned)
	spaceBetween := lipgloss.Width(summaryTableRendered) - lipgloss.Width(totalCountsRendered) - lipgloss.Width(pageCountsRendered)

	var metaDataRendered string

	// If the spaceBetween the left-aligned and the right-aligned content is too little, display them underneath each other.
	// (5 is an arbitrary number in this case)
	if spaceBetween <= 5 {
		metaDataRendered = pageCountsRendered + "\n" + lipgloss.NewStyle().PaddingLeft(m.styles.NameHeader.GetPaddingLeft()).Render(totalCountsRendered)
	} else {
		metaDataRendered = pageCountsRendered + strings.Repeat(" ", spaceBetween) + lipgloss.NewStyle().PaddingRight(m.styles.NameHeader.GetPaddingRight()).Render(totalCountsRendered)
	}

	summary := m.styles.Border.Render(lipgloss.JoinVertical(lipgloss.Left, summaryTableRendered, metaDataRendered))

	// If there is more than one page, render the "scroll bar"
	if len(m.rows) > m.pageSize {
		summary = lipgloss.JoinHorizontal(
			lipgloss.Center,
			summary,
			renderPageScrollBar(m.pageCurrentStart, m.pageCurrentEnd, len(m.rows), m.Help.Styles.FullDesc, m.Help.Styles.FullKey, m.KeyMap.PageUpSummary.Keys()[0], m.KeyMap.PageDownSummary.Keys()[0]),
		)
	}

	output := m.renderOutput()

	return lipgloss.JoinVertical(lipgloss.Left, summary, output) + "\n" + m.Help.View(m.KeyMap)
}

// headerView renders the content for the summary table's headers.
func (m Model) headerView() string {
	// s is set to the length of name column + task columns + prefixColumns
	s := make([]string, 0, len(m.taskColumns)+len(m.prefixColumns)+1)

	// render the name column cell
	style := lipgloss.NewStyle().Width(m.nameColumnWidth).MaxWidth(m.nameColumnWidth).Inline(true)
	renderedCell := style.Render("Name")
	s = append(s, m.styles.NameHeader.Render(renderedCell))

	// render the prefix columns
	for _, col := range m.prefixColumns {
		style = lipgloss.NewStyle().Width(col.Width).MaxWidth(col.Width).Inline(true)
		renderedCell = style.Render(col.Name)
		s = append(s, m.styles.TaskHeaders.Render(renderedCell))
	}

	// render the task columns
	for _, col := range m.taskColumns {
		style = lipgloss.NewStyle().Width(col.Width).MaxWidth(col.Width).Inline(true)
		renderedCell = style.Render(col.Name)
		s = append(s, m.styles.TaskHeaders.Render(renderedCell))
	}

	return lipgloss.JoinHorizontal(lipgloss.Top, s...)
}

// renderRow renders the content for a row with index i.
func (m Model) renderRow(i int) string {
	s := make([]string, 0, len(m.taskColumns)+len(m.prefixColumns)+1)

	style := lipgloss.NewStyle().Width(m.nameColumnWidth).MaxWidth(m.nameColumnWidth).Inline(true)
	renderedCell := style.Render(m.rows[i].Name)

	// if the currently selected output msg is on a task for this row, then use m.styles.Selected to indicate it to the user.
	if selectedItem, _, _ := m.selector.GetSelected(); m.showOutput && selectedItem != nil && selectedItem.Index == i {
		s = append(s, m.styles.Cell.Inherit(m.styles.Selected).Render(renderedCell))
	} else {
		s = append(s, m.styles.Cell.Render(renderedCell))
	}

	for k, col := range m.prefixColumns {
		style = lipgloss.NewStyle().Width(col.Width).MaxWidth(col.Width).Inline(true).Align(lipgloss.Left)
		s = append(s, m.renderPrefixColumn(m.rows[i].PrefixValues[k], style, i))
	}

	for k, col := range m.taskColumns {
		if customStatus := m.rows[i].Tasks[k].customSuccessStatus; customStatus.Message != "" {

			style = lipgloss.NewStyle().Width(col.Width).MaxWidth(col.Width).Inline(true).Align(lipgloss.Left)

			var r string

			if selectedItem, row, icol := m.selector.GetSelected(); m.showOutput && selectedItem != nil && row == i && k == icol {
				r = m.styles.Cell.Inherit(m.styles.Selected).Render(style.Render(customStatus.Message))
			} else {
				if customStatus.ForegroundColour != "" {
					style = style.Foreground(lipgloss.Color(customStatus.ForegroundColour))
				}
				r = m.styles.Cell.Render(style.Render(customStatus.Message))
			}

			s = append(s, r)

		} else {
			style = lipgloss.NewStyle().Width(col.Width).MaxWidth(col.Width).Inline(true).Align(lipgloss.Center)
			s = append(s, m.renderTaskColumn(m.rows[i].Tasks[k].Status, style, i, k))
		}
	}

	return lipgloss.JoinHorizontal(lipgloss.Top, s...)
}

func (m Model) renderPrefixColumn(value string, style lipgloss.Style, index int) string {
	renderedCell := style.Render(value)

	if _, row, _ := m.selector.GetSelected(); m.showOutput && row == index {
		return m.styles.Cell.Inherit(m.styles.Selected).Render(renderedCell)
	}

	return m.styles.Cell.Render(renderedCell)
}

// renderColumn aligns and colors the status symbols for the task columns.
func (m Model) renderTaskColumn(status TaskStatus, style lipgloss.Style, index, taskIndex int) string {
	var r, renderedCell string

	switch status {
	case Success:
		renderedCell = style.Render("✓")

		// shows the user which task output they are looking at.
		if selectedItem, row, col := m.selector.GetSelected(); m.showOutput && selectedItem != nil && row == index && col == taskIndex {
			return m.styles.Cell.Inherit(m.styles.Selected).Render(renderedCell)
		}

		return m.styles.Cell.Foreground(lipgloss.Color("42")).Render(renderedCell)
	case Failure:
		renderedCell = style.Render("✗")

		// shows the user which task output they are looking at.
		if selectedItem, row, col := m.selector.GetSelected(); m.showOutput && selectedItem != nil && row == index && col == taskIndex {
			return m.styles.Cell.Inherit(m.styles.Selected).Render(renderedCell)
		}
		return m.styles.Cell.Foreground(lipgloss.Color("9")).Render(renderedCell)
	case Skip:
		renderedCell = style.Render("-")
		return m.styles.Cell.Foreground(lipgloss.Color("#767676")).Render(renderedCell)
	case InProgress:
		renderedCell = style.Render(m.spinner.View())
		return m.styles.Cell.Render(renderedCell)
	case NotStarted:
		renderedCell = style.Render("!")
		return m.styles.Cell.Foreground(lipgloss.Color("#767676")).Render(renderedCell)
	}
	return r
}

func (m Model) renderTotalCounts() string {
	var s string

	isFailureCount, isInProgressCount, isSuccessCount := m.failureCount > 0, m.inProgressCount > 0, m.successCount > 0

	if isFailureCount {
		s += "✗ " + strconv.Itoa(m.failureCount)

		if isInProgressCount || isSuccessCount {
			s += ", "
		}
	}

	if isInProgressCount {
		s += "⣯ " + strconv.Itoa(m.inProgressCount)

		if isSuccessCount {
			s += ", "
		}
	}

	if isSuccessCount {
		s += "✓ " + strconv.Itoa(m.successCount)
	}

	return lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{
		Light: "#909090",
		Dark:  "#626262",
	}).Render(fmt.Sprintf("total: (%s) / %d", s, len(m.rows)))
}

func (m Model) renderPageCounts() string {
	numPages, currPage, _, _ := GetPaginationDetails(len(m.rows), m.pageSize, m.pageCurrentStart)

	return lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{
		Light: "#909090",
		Dark:  "#626262",
	}).PaddingLeft(m.styles.NameHeader.GetPaddingLeft()).Render(fmt.Sprintf("page: %d of %d", currPage+1, numPages))
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
		// output = m.styles.Border.Render(fmt.Sprintf("%s\n%s\n", lipgloss.NewStyle().Bold(true).Render("Output"), lipgloss.NewStyle().Foreground(m.styles.Border.GetBorderBottomForeground()).Render(strings.Repeat("─", m.output.viewport.Width*3))) + m.output.View())
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

// ShortHelp implements the KeyMap interface.
func (km KeyMap) ShortHelp() []key.Binding {
	return []key.Binding{km.Quit, km.Help, km.ShowOutput, km.LineUp, km.LineDown, km.ColLeft, km.ColRight}
}

// FullHelp implements the KeyMap interface.
func (km KeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{km.Quit, km.Help, km.ShowOutput, km.LineUp, km.LineDown},
		{km.Down, km.Up, km.HalfPageDown, km.HalfPageUp, km.PageDownSummary},
		{km.PageUpSummary, km.ColLeft, km.ColRight, km.PageUp, km.PageDown},
	}
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
			key.WithDisabled(),
		),
		LineDown: key.NewBinding(
			key.WithKeys("down"),
			key.WithHelp("↓", "down"),
			key.WithDisabled(),
		),
		PageDownSummary: key.NewBinding(
			key.WithKeys("g"),
			key.WithHelp("g", "page down"),
		),
		PageUpSummary: key.NewBinding(
			key.WithKeys("t"),
			key.WithHelp("t", "page down"),
		),
		ColLeft: key.NewBinding(
			key.WithKeys("left"),
			key.WithHelp("←", "left"),
			key.WithDisabled(),
		),
		ColRight: key.NewBinding(
			key.WithKeys("right"),
			key.WithHelp("→", "right"),
			key.WithDisabled(),
		),
		ShowOutput: key.NewBinding(
			key.WithKeys("s"),
			key.WithHelp("s", "toggle logs"),
			key.WithDisabled(),
		),
		PageDown: key.NewBinding(
			// key.WithKeys("pgdown", spacebar, "f"),
			key.WithKeys("pgdown", "f"),
			key.WithHelp("f/pgdn", "page down"),
		),
		PageUp: key.NewBinding(
			key.WithKeys("pgup", "b"),
			key.WithHelp("b/pgup", "page up"),
		),
		HalfPageUp: key.NewBinding(
			key.WithKeys("u", "ctrl+u"),
			key.WithHelp("u", "½ page up"),
		),
		HalfPageDown: key.NewBinding(
			key.WithKeys("d", "ctrl+d"),
			key.WithHelp("d", "½ page down"),
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

func GetPaginationDetails(totalElements, elementsPerPage, currentElementIndex int) (numPages int, currentPage int, pageStartIndex int, pageEndIndex int) {
	numPages = int(math.Ceil(float64(totalElements) / float64(elementsPerPage)))
	pageStartIndex = (currentElementIndex / elementsPerPage) * elementsPerPage
	pageEndIndex = int(math.Min(float64(pageStartIndex+elementsPerPage-1), float64(totalElements-1)))
	currentPage = int(math.Ceil(float64(pageStartIndex) / float64(elementsPerPage)))
	return
}
