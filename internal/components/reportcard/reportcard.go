package reportcard

import (
	"math"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/viewport"
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
	spacebar = " "

	// Minimum width of the name column. (the name column is sized dynamically based on the longest name)
	minNameLength = 4
)

type TaskResultMsg struct {
	Status    TaskStatus
	Index     int    // Index is the index of the item in Model.Rows
	taskIndex int    // Gets set internally after a failure. This is used to match failures to specific tasks on the frontend.
	Output    string // Should be set on failure. Will be displayed to the end user.
	IsFatal   bool   // Skip all following tasks
}

type Task struct {
	Status TaskStatus
	// ShouldRunAnywayFor allows a task to "run" on the reportcard regardless of if a previous task
	// failed fatally. ShouldRunAnywayFor is a map[int]bool where the keys are task indexes and the
	// values are bools indicating whether the task should run for said task.
	ShouldRunAnywayFor map[int]bool
}

type Row struct {
	Name         string
	Tasks        []Task
	PrefixValues []string
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
	LineUp     key.Binding
	LineDown   key.Binding
	ColLeft    key.Binding
	ColRight   key.Binding
	ShowOutput key.Binding
	Quit       key.Binding
	Help       key.Binding

	// Output
	PageDown     key.Binding
	PageUp       key.Binding
	HalfPageUp   key.Binding
	HalfPageDown key.Binding
	Down         key.Binding
	Up           key.Binding
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
	output          output
	// activeTasks is a map[int]int where the key is an index for Model.rows and the value is the index for Model.rows[n].Tasks[].
	// activeTasks stores which tasks are currently in progress for all of the items. Entries are deleted once their tasks are finished.
	activeTasks           map[int]int
	styles                Styles
	canShowOutput         bool // an error has occurred for one of the tasks and there is output available that can be shown to the user.
	showOutput            bool // showOutput indicates whether the output should be shown.
	selector              selector[TaskResultMsg]
	termWidth             int // termWidth contains the width of the terminal. This is used to dynamically size the output window.
	notFirstCompletedTask bool
}

type cursorDirection int

const (
	up cursorDirection = iota
	right
	down
	left
)

// selector stores all of the results and handles logic for moving between selected results.
type selector[T any] struct {
	notFirst           bool
	selectedItemRow    int
	selectedItemColumn int
	selectableItems    [][]*T // using pointer to easily check if selected
}

func (s *selector[T]) MoveCursor(direction cursorDirection) (T, int, int) {
	switch direction {
	case up:
	upOuter:
		for i := s.selectedItemRow - 1; i >= 0; i-- {
			if s.selectableItems[i][s.selectedItemColumn] != nil {
				s.selectedItemRow = i
			} else {
				for j := range len(s.selectableItems[i]) {
					if s.selectableItems[i][j] != nil {
						s.selectedItemColumn = j
						s.selectedItemRow = i
						break upOuter
					}
				}
			}
		}
	case down:
	downOuter:
		for i := s.selectedItemRow + 1; i < len(s.selectableItems); i++ {
			if s.selectableItems[i][s.selectedItemColumn] != nil {
				s.selectedItemRow = i
			} else {
				for j := range len(s.selectableItems[i]) {
					if s.selectableItems[i][j] != nil {
						s.selectedItemColumn = j
						s.selectedItemRow = i
						break downOuter
					}
				}
			}
		}
	case right:
		for i := s.selectedItemColumn + 1; i < len(s.selectableItems[s.selectedItemRow]); i++ {
			if s.selectableItems[s.selectedItemRow][i] != nil {
				s.selectedItemColumn = i
				break
			}
		}
	case left:
		for i := s.selectedItemColumn - 1; i >= 0; i-- {
			if s.selectableItems[s.selectedItemRow][i] != nil {
				s.selectedItemColumn = i
				break
			}
		}
	}

	return *s.selectableItems[s.selectedItemRow][s.selectedItemColumn], s.selectedItemRow, s.selectedItemColumn
}

func (s *selector[T]) AddSelectable(item T, row, col int) {
	s.selectableItems[row][col] = &item

	if !s.notFirst {
		s.selectedItemRow, s.selectedItemColumn = row, col
		s.notFirst = true
	}
}

func (s *selector[T]) GetSelected() (item *T, row, col int) {
	item, row, col = s.selectableItems[s.selectedItemRow][s.selectedItemColumn], s.selectedItemRow, s.selectedItemColumn
	return
}

func newSelector[T any](numColumns, numRows int) selector[T] {
	s := make([][]*T, numRows)

	for i := range s {
		s[i] = make([]*T, numColumns)
	}

	return selector[T]{
		selectableItems: s,
	}
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

func NewModel(options ...Option) Model {
	m := Model{
		nameColumnWidth: minNameLength,
		activeTasks:     make(map[int]int),
		KeyMap:          DefaultKeyMap(),
		Help:            help.New(),
	}

	for _, opt := range options {
		opt(&m)
	}

	m.selector = newSelector[TaskResultMsg](len(m.taskColumns), len(m.rows))

	// If a spinner wasn't set, set a default spinner.
	if m.spinner.ID() == 0 {
		m.spinner = spinner.New()
	}

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
		// get the width of the terminal
		m.termWidth = msg.Width
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, m.KeyMap.Quit):
			return m, tea.Quit

		case key.Matches(msg, m.KeyMap.Help):
			m.Help.ShowAll = !m.Help.ShowAll

		case key.Matches(msg, m.KeyMap.ColLeft):
			// If there is output to show, move left on the table.
			item, _, _ := m.selector.MoveCursor(left)
			m.output.SetContent(item.Output)

		case key.Matches(msg, m.KeyMap.ColRight):
			// If there is output to show, move right on the table.
			item, _, _ := m.selector.MoveCursor(right)
			m.output.SetContent(item.Output)

		case key.Matches(msg, m.KeyMap.LineDown):
			// If there is output to show, move down on the table.
			item, _, _ := m.selector.MoveCursor(down)
			m.output.SetContent(item.Output)

		case key.Matches(msg, m.KeyMap.LineUp):
			// If there is output to show, move down on the table.
			item, _, _ := m.selector.MoveCursor(up)
			m.output.SetContent(item.Output)

		case key.Matches(msg, m.KeyMap.ShowOutput):
			// If there is output to show, show/hide the output.
			m.showOutput = !m.showOutput

		case key.Matches(msg, m.KeyMap.PageDown):
			lines := m.output.viewport.ViewDown()
			if m.output.viewport.HighPerformanceRendering {
				cmds = append(cmds, viewport.ViewDown(m.output.viewport, lines))
			}

		case key.Matches(msg, m.KeyMap.PageUp):
			lines := m.output.viewport.ViewUp()
			if m.output.viewport.HighPerformanceRendering {
				cmds = append(cmds, viewport.ViewUp(m.output.viewport, lines))
			}

		case key.Matches(msg, m.KeyMap.HalfPageDown):
			lines := m.output.viewport.HalfViewDown()
			if m.output.viewport.HighPerformanceRendering {
				cmds = append(cmds, viewport.ViewDown(m.output.viewport, lines))
			}

		case key.Matches(msg, m.KeyMap.HalfPageUp):
			lines := m.output.viewport.HalfViewUp()
			if m.output.viewport.HighPerformanceRendering {
				cmds = append(cmds, viewport.ViewUp(m.output.viewport, lines))
			}

		case key.Matches(msg, m.KeyMap.Down):
			lines := m.output.viewport.LineDown(1)
			if m.output.viewport.HighPerformanceRendering {
				cmds = append(cmds, viewport.ViewDown(m.output.viewport, lines))
			}

		case key.Matches(msg, m.KeyMap.Up):
			lines := m.output.viewport.LineUp(1)
			if m.output.viewport.HighPerformanceRendering {
				cmds = append(cmds, viewport.ViewUp(m.output.viewport, lines))
			}
		}
	case TaskResultMsg:
		// if the incoming msg is for a task that is still active, handle it, otherwise
		// ignore it. This handles the scenario where the sender continues to send msgs
		// after all tasks should be in a final status.
		if taskIndex, ok := m.activeTasks[msg.Index]; ok {
			if !m.notFirstCompletedTask {
				// for that first task that completes, enable keys and set the output content.
				m.setCanShowOutput()
				m.output.SetContent(msg.Output)
				m.notFirstCompletedTask = true
			}
			msg.taskIndex = taskIndex
			m.selector.AddSelectable(msg, msg.Index, msg.taskIndex)

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
	m.output, cmd = m.output.Update(msg)
	cmds = append(cmds, cmd)
	return m, tea.Batch(cmds...)
}

// setCanShowOutput allows the user to toggle on/of the error/standard output for the tasks.
func (m *Model) setCanShowOutput() {
	m.canShowOutput = true
	m.KeyMap.ShowOutput.SetEnabled(true)
}

func (m *Model) updateAvailableKeys() {
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
func (m Model) handleRemainingTasks(i int, taskIndex int, isFatal bool) {
	var s bool
	for k := taskIndex; k < len(m.rows[i].Tasks); k++ {
		if m.rows[i].Tasks[k].Status == NotStarted {
			m.activeTasks[i] = k

			if isFatal {
				if m.rows[i].Tasks[k].ShouldRunAnywayFor[taskIndex] {
					// If the original task failed fatally, but the current
					// task is allowed to run anyway, change its status to
					// InProgress
					m.rows[i].Tasks[k].Status = InProgress
					s = true
				} else {
					m.rows[i].Tasks[k].Status = Skip
				}
			} else {
				m.rows[i].Tasks[k].Status = InProgress
				s = true
				break
			}
		}
	}

	if !s {
		delete(m.activeTasks, i)
	}
}

func (m Model) View() string {
	rows := make([]string, 0, len(m.rows)+1)
	rows = append(rows, m.headerView())
	for i := range m.rows {
		rows = append(rows, m.renderRow(i))
	}

	summary := m.styles.Border.Render(lipgloss.JoinVertical(lipgloss.Top, rows...))

	var output string
	if m.showOutput {
		output = m.styles.Border.Render(lipgloss.NewStyle().Bold(true).Render("Output") + "\n" + m.output.View())
	}
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
		style = lipgloss.NewStyle().Width(col.Width).MaxWidth(col.Width).Inline(true).Align(lipgloss.Center)
		s = append(s, m.renderTaskColumn(m.rows[i].Tasks[k].Status, style, i, k))
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

// ShortHelp implements the KeyMap interface.
func (km KeyMap) ShortHelp() []key.Binding {
	return []key.Binding{km.Quit, km.Help, km.ShowOutput, km.LineUp, km.LineDown, km.ColLeft, km.ColRight, km.Down, km.Up}
}

// FullHelp implements the KeyMap interface.
func (km KeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{km.Quit, km.Help, km.ShowOutput, km.LineUp, km.LineDown},
		{km.Down, km.Up, km.HalfPageDown, km.HalfPageUp, km.LineUp},
		{km.LineDown, km.ColLeft, km.ColRight, km.PageUp, km.PageDown},
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
			key.WithDisabled(),
		),
		LineDown: key.NewBinding(
			key.WithKeys("down"),
			key.WithHelp("↓", "down"),
			key.WithDisabled(),
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
			key.WithHelp("s", "show logs"),
			key.WithDisabled(),
		),
		PageDown: key.NewBinding(
			key.WithKeys("pgdown", spacebar, "f"),
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
