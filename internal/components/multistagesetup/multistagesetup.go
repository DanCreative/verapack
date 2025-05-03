package multistagesetup

import (
	"fmt"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type SetupTaskStatus int

var (
	symbolStyle          = lipgloss.NewStyle().Width(5)
	blockWidthPercentage = 0.75 // percentage of terminal width that the stage block will take up.
)

const (
	SetupTaskSuccess    SetupTaskStatus = iota // Task was successful (Final Status)
	SetupTaskFailure                           // Task failed. This will cause all following tasks to skip (Final Status)
	SetupTaskWarning                           // Task is successful, but with warning(s) (Final Status)
	SetupTaskInProgress                        // Task is in progress. Shows the view of the task model
	SetupTaskTodo                              // Task is still to come
	SetupTaskSkipped                           // Task is not required to be run
)

// taskResult implements interface tea.Msg and is returned to the tea runtime and by extension setupModel.Update()
type TaskResult struct {
	Status SetupTaskStatus
	Err    error
	Msg    string
}

func NewFailedTaskResult(msg string, err error) TaskResult {
	return TaskResult{Status: SetupTaskFailure, Msg: msg, Err: err}
}

func NewSuccessfulTaskResult(msg string) TaskResult {
	return TaskResult{Status: SetupTaskSuccess, Msg: msg}
}

func NewSkippedTaskResult(msg string) TaskResult {
	return TaskResult{Status: SetupTaskSkipped, Msg: msg}
}

func NewWarningTaskResult(msg string) TaskResult {
	return TaskResult{Status: SetupTaskWarning, Msg: msg}
}

// SetupTask wraps the teaTasker and contains any data that the setupModel will use.
type SetupTask struct {
	status  SetupTaskStatus
	summary string // Summary of task
	msg     string
	err     error
	task    TeaTasker
}

func NewSetupTask(initialMsg string, task TeaTasker) SetupTask {
	return SetupTask{
		summary: initialMsg,
		status:  SetupTaskTodo,
		task:    task,
	}
}

// interface teaTasker merges tea.Model with custom methods.
type TeaTasker interface {
	tea.Model

	// GetHelp returns any help instructions specific to that task.
	GetHelp() help.KeyMap
}

type KeyMap struct {
	Quit key.Binding
	Help key.Binding
}

func (k KeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Quit, k.Help}
}

func (k KeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{{k.Quit, k.Help}}
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
			key.WithDisabled(),
		),
	}
}

// Option is used to set options in New. For example:
//
//	setup := New(WithTasks([]Column{{Title: "ID", Width: 10}}))
type Option func(*Model)

type Styles struct {
	StatusFailure    SummaryStyle
	StatusSuccess    SummaryStyle
	StatusInProgress lipgloss.Style
	StatusSkipped    SummaryStyle
	StatusTodo       SummaryStyle
	StatusWarning    SummaryStyle
	StageBlock       lipgloss.Style
	MsgText          lipgloss.Style
	FinalMessage     lipgloss.Style
}

type SummaryStyle struct {
	Style  lipgloss.Style
	Symbol rune
	Colour lipgloss.Color
}

// render returns the rendered symbol and summary for a task in Model.tasks
func (s *SummaryStyle) render(taskmsg string) (string, string) {
	return symbolStyle.Foreground(s.Colour).Render(string(s.Symbol)), s.Style.Render(taskmsg)
}

// Model is the parent tea.Model that will be run.
// All tasks contain a child tea.Model with its own Init, Update and View methods.
type Model struct {
	spinner            spinner.Model
	tasks              []SetupTask
	activeTask         int
	KeyMap             KeyMap
	Help               help.Model
	styles             Styles
	termWidth          int
	finalMessage       string
	isSuccessfullyDone bool
}

// Init sets the first task's status to in progress, runs the first task's Init method and starts the spinner
func (m Model) Init() tea.Cmd {
	if len(m.tasks) < 1 {
		return nil
	}
	m.tasks[m.activeTask].status = SetupTaskInProgress
	m.setTaskFullHelpAvailable()
	return tea.Batch(m.spinner.Tick, m.tasks[m.activeTask].task.Init())
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if len(m.tasks) < 1 {
		return m, tea.Quit
	}

	var newTask bool

	switch msgt := msg.(type) {
	case tea.WindowSizeMsg:
		m.termWidth = msgt.Width
	case tea.KeyMsg:
		// If key is disabled, Matches() will return false
		switch {
		case key.Matches(msgt, m.KeyMap.Quit):
			return m, tea.Quit
		case key.Matches(msgt, m.KeyMap.Help):
			m.Help.ShowAll = !m.Help.ShowAll

			// Setting msg to nil so that it is not passed down to children
			// i.e. adding '?' to input when the user wants to view additional
			// help.
			// May remove in future if required.
			msg = nil
		}

	case TaskResult:
		m.tasks[m.activeTask].msg = msgt.Msg
		switch msgt.Status {
		case SetupTaskSuccess, SetupTaskWarning, SetupTaskSkipped:
			m.tasks[m.activeTask].status = msgt.Status

			if m.activeTask+1 < len(m.tasks) {
				// There are still more tasks in the list
				m.activeTask++
				m.tasks[m.activeTask].status = SetupTaskInProgress
				newTask = true
				m.setTaskFullHelpAvailable()
			} else {
				// There are no more tasks
				m.isSuccessfullyDone = true
				return m, tea.Quit
			}
		case SetupTaskFailure:
			m.tasks[m.activeTask].status = SetupTaskFailure
			m.tasks[m.activeTask].err = msgt.Err

			// re-run update and view one more time before quitting
			return m, tea.Sequence(
				func() tea.Msg { return nil }, tea.Quit,
			)
		}
	}

	cmds := make([]tea.Cmd, 2)

	var task tea.Model

	if newTask {
		// if this is the first time this task is run, run the Init() method
		cmds[0] = m.tasks[m.activeTask].task.Init()
	} else {
		// Otherwise, run the normal Update() method
		task, cmds[0] = m.tasks[m.activeTask].task.Update(msg)

		// task will always be a teaTasker
		m.tasks[m.activeTask].task = task.(TeaTasker)
	}

	m.spinner, cmds[1] = m.spinner.Update(msg)

	return m, tea.Batch(cmds...)
}

func (m *Model) setTaskFullHelpAvailable() {
	if len(m.getTaskFullHelp()) > 0 {
		m.KeyMap.Help.SetEnabled(true)
	} else {
		m.KeyMap.Help.SetEnabled(false)
	}
}

func (m *Model) getTaskShortHelp() []key.Binding {
	if h := m.tasks[m.activeTask].task.GetHelp(); h != nil {
		return h.ShortHelp()
	}
	return nil
}

func (m *Model) getTaskFullHelp() [][]key.Binding {
	if h := m.tasks[m.activeTask].task.GetHelp(); h != nil {
		return h.FullHelp()
	}
	return nil
}

func (m Model) View() string {
	if len(m.tasks) < 1 {
		return lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{
			Light: "#909090",
			Dark:  "#626262",
		}).Render("No Tasks")
	}

	var s string

	for _, task := range m.tasks {
		var symbol, summary, msg, block string

		if task.msg != "" {
			msg = m.styles.MsgText.Render("(" + task.msg + ")")
		}

		switch task.status {
		case SetupTaskTodo:
			symbol, summary = m.styles.StatusTodo.render(task.summary)

		case SetupTaskInProgress:
			symbol, summary = symbolStyle.Render(m.spinner.View()), m.styles.StatusInProgress.Render(task.summary)
			if b := m.tasks[m.activeTask].task.View(); len(b) > 0 {
				block = m.styles.StageBlock.Width(int(float64(m.termWidth)*blockWidthPercentage)).Render(b) + "\n"
			}

		case SetupTaskWarning:
			symbol, summary = m.styles.StatusWarning.render(task.summary)

		case SetupTaskSuccess:
			symbol, summary = m.styles.StatusSuccess.render(task.summary)

		case SetupTaskFailure:
			symbol, summary = m.styles.StatusFailure.render(task.summary)
			block = m.styles.StageBlock.Render(task.err.Error()) + "\n"

		case SetupTaskSkipped:
			symbol, summary = m.styles.StatusSkipped.render(task.summary)
		}

		s += fmt.Sprintf("%s%s %s\n%s", symbol, summary, msg, block)
	}

	if m.isSuccessfullyDone {
		s += m.styles.FinalMessage.Width(int(float64(m.termWidth)*blockWidthPercentage)).Render(m.finalMessage) + "\n"
	}

	// if showing short help, parent help will be added to the front
	// if showing full help, parent help will be set as the first column
	if m.Help.ShowAll {
		s += m.Help.FullHelpView(append(m.KeyMap.FullHelp(), m.getTaskFullHelp()...))
	} else {
		s += m.Help.ShortHelpView(append(m.KeyMap.ShortHelp(), m.getTaskShortHelp()...))
	}

	return s
}

func NewModel(options ...Option) Model {
	m := Model{
		KeyMap: DefaultKeyMap(),
		Help:   help.New(),
	}

	for _, opt := range options {
		opt(&m)
	}

	// If a spinner wasn't set, set a default spinner.
	if m.spinner.ID() == 0 {
		m.spinner = spinner.New()
	}

	return m
}

// WithStyles sets the report card's styles.
func WithStyles(styles Styles) Option {
	return func(m *Model) {
		m.styles = styles
	}
}

func WithTasks(tasks ...SetupTask) Option {
	return func(m *Model) {
		m.tasks = tasks
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

func WithFinalMessage(s string) Option {
	return func(m *Model) {
		m.finalMessage = s
	}
}
