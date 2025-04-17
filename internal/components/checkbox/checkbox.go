package checkbox

import (
	"fmt"
	"strconv"

	"github.com/charmbracelet/bubbles/cursor"
	tea "github.com/charmbracelet/bubbletea"
)

// Checkbox input for handling bool values.
type Model struct {
	Cursor cursor.Model
	value  bool
	focus  bool
	Err    error
}

func (m *Model) CursorEnd() {}

func (m *Model) SetValue(s string) {
	if v, err := strconv.ParseBool(s); err != nil {
		m.value = false
	} else {
		m.value = v
	}
}

func (m Model) Value() string {
	return strconv.FormatBool(m.value)
}

func (m *Model) Focus() tea.Cmd {
	m.focus = true
	return m.Cursor.Focus()
}

func (m *Model) Blur() {
	m.focus = false
	m.Cursor.Blur()
}

func (m *Model) Reset() {
	m.value = false
}

func (m Model) View() string {
	if m.value {
		m.Cursor.SetChar("x")
		return fmt.Sprintf("(%s)", m.Cursor.View())
	} else {
		m.Cursor.SetChar(" ")
		return fmt.Sprintf("(%s)", m.Cursor.View())
	}
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	if !m.focus {
		return m, nil
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case " ":
			m.value = !m.value
		}
	}

	var cmds []tea.Cmd
	var cmd tea.Cmd

	m.Cursor, cmd = m.Cursor.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

func New() Model {
	return Model{
		Cursor: cursor.New(),
		value:  false,
		focus:  false,
	}
}
