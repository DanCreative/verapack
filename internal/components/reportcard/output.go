package reportcard

import (
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
)

type output struct {
	ready    bool
	viewport viewport.Model
}

func (m *output) SetContent(s string) {
	m.viewport.SetContent(s)
}

func (m output) Init() tea.Cmd {
	return nil
}

func (m output) Update(msg tea.Msg) (output, tea.Cmd) {
	var (
		cmd  tea.Cmd
		cmds []tea.Cmd
	)

	switch msgt := msg.(type) {
	case tea.KeyMsg:
		msg = nil

	case tea.WindowSizeMsg:
		if !m.ready {
			m.viewport = viewport.New(int(float64(msgt.Width)*0.6), int(float64(msgt.Height)*0.3))
			m.viewport.YPosition = 1
			m.viewport.SetContent("")
			m.ready = true
		} else {
			m.viewport.Width = int(float64(msgt.Width) * 0.6)
			m.viewport.Height = int(float64(msgt.Height) * 0.3)
		}
	}

	// Handle keyboard and mouse events in the viewport
	m.viewport, cmd = m.viewport.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

func (m output) View() string {
	if !m.ready {
		// Should never show, output.View is only possible to run
		// once an error occurs. That will usually only happen after
		// a couple of mins.
		return "\n  Initializing..."
	}
	return m.viewport.View()
}
