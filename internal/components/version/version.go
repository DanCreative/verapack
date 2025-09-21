package version

import (
	"fmt"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type Styles struct {
	Muted lipgloss.Style // Used for tree lines and version numbers
	Loud  lipgloss.Style // Used for new version available indication
}

type VersionFunc func() (string, error)

type versionMsg struct {
	version     string
	app         string
	errOccurred bool
}

// Option is used to set options in NewModel.
type Option func(*Model)

type Model struct {
	appName                  string      // Name of the application
	wrapperLocalVersion      string      // Local Version of the wrapper
	packagerLocalVersion     string      // Local Version of the packager
	verapackLocalVersion     string      // Verapack Version
	wrapperLatestVersionMsg  versionMsg  // Latest version of the wrapper as well as whether an error occurred
	packagerLatestVersionMsg versionMsg  // Latest version of the packager as well as whether an error occurred
	wrapperVersionFunc       tea.Cmd     // tea.Cmd function to determine the latest wrapper version
	packagerVersionFunc      tea.Cmd     // tea.Cmd function to determine the latest packager version
	quitKey                  key.Binding // key.Binding for canceling the version check
	wasCancelled             bool        // set to true if the user cancelled the version check
	resultsReceived          int         // The number of results received. If this number matches the expectedResults, the model exits.
	expectedResults          int         // The number of expected results.
	spinner                  spinner.Model
	styles                   Styles
	help                     help.Model
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, m.packagerVersionFunc, m.wrapperVersionFunc)
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, m.quitKey):
			m.wasCancelled = true
			return m, tea.Quit
		}
	case versionMsg:
		switch {
		case msg.app == "packager":
			m.packagerLatestVersionMsg = msg

		case msg.app == "wrapper":
			m.wrapperLatestVersionMsg = msg

		}

		m.resultsReceived++

		if m.resultsReceived == m.expectedResults {
			return m, tea.Quit
		}
	}

	var cmd tea.Cmd
	m.spinner, cmd = m.spinner.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

func (m Model) View() string {
	return fmt.Sprintf("%s\n%s\n%s\n\n%s",
		fmt.Sprintf("%s version %s", m.appName, m.verapackLocalVersion),
		packagerVersionPrinter(m.packagerLatestVersionMsg, m),
		wrapperVersionPrinter(m.wrapperLatestVersionMsg, m),
		m.help.ShortHelpView([]key.Binding{m.quitKey}),
	)
}

// NewModel creates a new version tea model that prints the local version while checking for the latest version in the background.
//
// wrapperVersionFunc and packagerVersionFunc should return the version as a string and an error if one occurred.
func NewModel(wrapperLocalVersion, packagerLocalVersion, verapackLocalVersion string, appName string, wrapperVersionFunc, packagerVersionFunc VersionFunc, options ...Option) Model {
	packagerCmd := func() tea.Msg {
		msg := versionMsg{app: "packager"}
		version, err := packagerVersionFunc()
		if err != nil {
			msg.errOccurred = true

			return msg
		}

		msg.version = version

		return msg
	}

	wrapperCmd := func() tea.Msg {
		msg := versionMsg{app: "wrapper"}
		version, err := wrapperVersionFunc()
		if err != nil {
			msg.errOccurred = true

			return msg
		}

		msg.version = version

		return msg
	}

	m := Model{
		expectedResults:      2,
		wrapperLocalVersion:  wrapperLocalVersion,
		appName:              appName,
		packagerLocalVersion: packagerLocalVersion,
		verapackLocalVersion: verapackLocalVersion,
		packagerVersionFunc:  packagerCmd,
		wrapperVersionFunc:   wrapperCmd,
		quitKey:              key.NewBinding(key.WithKeys("ctrl+c"), key.WithHelp("ctrl+c", "quit")),
		help:                 help.New(),
	}

	for _, opt := range options {
		opt(&m)
	}

	return m
}

// WithStyles sets the report card's styles.
func WithStyles(styles Styles) Option {
	return func(m *Model) {
		m.styles = styles
	}
}

// WithSpinner sets the spinner options for the version display.
func WithSpinner(opts ...spinner.Option) Option {
	return func(m *Model) {
		m.spinner = spinner.New(opts...)
	}
}

func WithHelp(help help.Model) Option {
	return func(m *Model) {
		m.help = help
	}
}

func packagerVersionPrinter(msg versionMsg, m Model) string {
	var packagerVersion string
	if m.packagerLocalVersion == "na" {
		packagerVersion = fmt.Sprintf(" %s %s	(%s)",
			m.styles.Muted.Render("├──"),
			"Veracode CLI",
			m.styles.Muted.Render("not installed"),
		)
	} else {
		packagerVersion = fmt.Sprintf(" %s %s version %s",
			m.styles.Muted.Render("├──"),
			"Veracode CLI",
			m.packagerLocalVersion,
		)
	}

	if m.wasCancelled {
		return fmt.Sprintf("%s	(%s)",
			packagerVersion,
			m.styles.Muted.Render("cancelled by user"),
		)
	}

	if m.packagerLatestVersionMsg.app == "" {
		return fmt.Sprintf("%s	(%s %s)",
			packagerVersion,
			m.spinner.View(),
			m.styles.Muted.Render("checking latest version..."),
		)
	}

	if m.packagerLatestVersionMsg.errOccurred {
		return fmt.Sprintf("%s	(%s)",
			packagerVersion,
			m.styles.Muted.Render("an error has occurred, please try again later"),
		)
	}

	if m.packagerLatestVersionMsg.version == m.packagerLocalVersion {
		return fmt.Sprintf("%s	(%s)",
			packagerVersion,
			m.styles.Muted.Render("up to date"),
		)
	} else {
		return fmt.Sprintf("%s	(%s)",
			packagerVersion,
			m.styles.Loud.Render("a new version is available: "+msg.version),
		)
	}
}

func wrapperVersionPrinter(msg versionMsg, m Model) string {
	var wrapperVersion string
	if m.wrapperLocalVersion == "na" {
		wrapperVersion = fmt.Sprintf(" %s %s	(%s)",
			m.styles.Muted.Render("└──"),
			"VeracodeJavaAPI",
			m.styles.Muted.Render("not installed"),
		)
	} else {
		wrapperVersion = fmt.Sprintf(" %s %s version %s",
			m.styles.Muted.Render("└──"),
			"VeracodeJavaAPI",
			m.wrapperLocalVersion,
		)
	}

	if m.wasCancelled {
		return fmt.Sprintf("%s	(%s)",
			wrapperVersion,
			m.styles.Muted.Render("cancelled by user"),
		)
	}

	if m.wrapperLatestVersionMsg.app == "" {
		return fmt.Sprintf("%s	(%s %s)",
			wrapperVersion,
			m.spinner.View(),
			m.styles.Muted.Render("checking latest version..."),
		)
	}

	if m.wrapperLatestVersionMsg.errOccurred {
		return fmt.Sprintf("%s	(%s)",
			wrapperVersion,
			m.styles.Muted.Render("an error has occurred, please try again later"),
		)
	}

	if m.wrapperLatestVersionMsg.version == m.wrapperLocalVersion {
		return fmt.Sprintf("%s	(%s)",
			wrapperVersion,
			m.styles.Muted.Render("up to date"),
		)
	} else {
		return fmt.Sprintf("%s	(%s)",
			wrapperVersion,
			m.styles.Loud.Render("a new version is available: "+msg.version),
		)
	}
}
