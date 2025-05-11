package singleselect

import (
	"fmt"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type Styles struct {
	Highlight lipgloss.Style
	Border    lipgloss.Style
}

type PostFunc func(selection int, model Model) (tea.Model, tea.Cmd)

// singleselect.Model is a tea component that gives the user a list of options to select from.
// It also provides a function that can be run after the selection to perform an action based on the selection.
type Model struct {
	selected int
	options  []string
	bodyText string
	styles   Styles
	help     help.Model
	UpKey    key.Binding
	DownKey  key.Binding
	EnterKey key.Binding
	QuitKey  key.Binding
	postFunc PostFunc
}

type Option func(*Model)

func (m *Model) nextOption() {
	if m.options != nil {
		m.selected = (m.selected + 1) % len(m.options)
	}
}

func (m *Model) prevOption() {
	if m.options != nil {
		m.selected--
		// Wrap around
		if m.selected < 0 {
			m.selected = len(m.options) - 1
		}
	}
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, m.EnterKey):
			if m.postFunc == nil {
				return m, tea.Quit
			}

			return m.postFunc(m.selected, m)

		case key.Matches(msg, m.QuitKey):
			return m, tea.Quit

		case key.Matches(msg, m.DownKey):
			m.nextOption()

		case key.Matches(msg, m.UpKey):
			m.prevOption()

		}
	}

	return m, nil
}

func (m Model) View() string {
	s := m.bodyText + "\n\n"

	for k, text := range m.options {
		if k == m.selected {
			s += fmt.Sprintf("(%s)  %s", m.styles.Highlight.Render("x"), m.styles.Highlight.Render(text))
		} else {
			s += "( )  " + text
		}

		if k != len(m.options)-1 {
			s += "\n"
		}
	}

	return m.styles.Border.Render(s) + "\n" + m.help.ShortHelpView([]key.Binding{m.QuitKey, m.UpKey, m.DownKey, m.EnterKey})
}

func NewModel(options ...Option) Model {
	m := Model{
		help: help.New(),
		EnterKey: key.NewBinding(
			key.WithHelp("enter", "submit"),
			key.WithKeys("enter"),
		),
		UpKey: key.NewBinding(
			key.WithHelp("↑/up", "up"),
			key.WithKeys("up"),
		),
		DownKey: key.NewBinding(
			key.WithHelp("↓/down", "down"),
			key.WithKeys("down"),
		),
		QuitKey: key.NewBinding(
			key.WithKeys("q", "esc", "ctrl+c"),
			key.WithHelp("ctrl+c", "quit"),
		),
	}

	for _, opt := range options {
		opt(&m)
	}

	return m
}

// WithStyles sets the styles for the multiselect.
func WithStyles(styles Styles) Option {
	return func(m *Model) {
		m.styles = styles
	}
}

// WithOptions sets the options for the multiselect.
func WithOptions(options ...string) Option {
	return func(m *Model) {
		m.options = options
	}
}

// WithOptions sets the body text for the multiselect.
func WithBodyText(text string) Option {
	return func(m *Model) {
		m.bodyText = text
	}
}

func WithPostFunc(postFunc PostFunc) Option {
	return func(m *Model) {
		m.postFunc = postFunc
	}
}
