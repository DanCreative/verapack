package verapack

import (
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/lipgloss"
)

var (
	lightBlue = lipgloss.Color("#00b3e6")
	// darkBlue  = lipgloss.Color("#0E78F0")
	darkGray = lipgloss.Color("#767676")
	green    = lipgloss.Color("42")
	red      = lipgloss.Color("9")
	orange   = lipgloss.Color("#FFA500")

	redForeground       = lipgloss.NewStyle().Foreground(red)
	lightBlueForeground = lipgloss.NewStyle().Foreground(lightBlue)
	// darkBlueForeground  = lipgloss.NewStyle().Foreground(darkBlue)
	darkGrayForeground = lipgloss.NewStyle().Foreground(darkGray)

	defaultSpinnerOpts = []spinner.Option{
		spinner.WithSpinner(spinner.Spinner{
			Frames: []string{"⣾", "⣽", "⣻", "⢿", "⡿", "⣟", "⣯", "⣷"},
			FPS:    time.Second / 10,
		}),
		spinner.WithStyle(lightBlueForeground),
	}
)
