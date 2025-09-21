package verapack

import (
	"time"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/lipgloss"
)

var (
	lightBlue = lipgloss.Color("#00b3e6")
	darkGray  = lipgloss.Color("#767676")
	green     = lipgloss.Color("42")
	red       = lipgloss.Color("9")
	orange    = lipgloss.Color("#FFA500")

	redForeground       = lipgloss.NewStyle().Foreground(red)
	lightBlueForeground = lipgloss.NewStyle().Foreground(lightBlue)
	darkGrayForeground  = lipgloss.NewStyle().Foreground(darkGray)

	defaultSpinnerOpts = []spinner.Option{
		spinner.WithSpinner(spinner.Spinner{
			Frames: []string{"⣾", "⣽", "⣻", "⢿", "⡿", "⣟", "⣯", "⣷"},
			FPS:    time.Second / 10,
		}),
		spinner.WithStyle(lightBlueForeground),
	}

	keyStyle  = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#909090", Dark: "#b8b6b6ff"})
	descStyle = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#B2B2B2", Dark: "#4A4A4A"})
	sepStyle  = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#DDDADA", Dark: "#3C3C3C"})

	defaultHelp = help.Model{
		ShortSeparator: " • ",
		FullSeparator:  "    ",
		Ellipsis:       "…",
		Styles: help.Styles{
			ShortKey:       keyStyle,
			ShortDesc:      descStyle,
			ShortSeparator: sepStyle,
			Ellipsis:       sepStyle,
			FullKey:        keyStyle,
			FullDesc:       descStyle,
			FullSeparator:  sepStyle,
		},
	}
)
