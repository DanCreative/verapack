package verapack

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/DanCreative/veracode-go/veracode"
	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const (
	credentialFileLegacyFormat = "[default]\nveracode_api_key_id     = %s\nveracode_api_key_secret = %s"
	credentialFileFormat       = "api:\n  key-id: %s\n  key-secret: %s"
)

// setLegacyCredentialsFile creates/truncates the credential file: %home%/.veracode/credential
func setLegacyCredentialsFile(homeDir, apiKey, apiSecret string) error {
	file, err := os.Create(filepath.Join(homeDir, ".veracode", "credentials"))
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = file.WriteString(fmt.Sprintf(credentialFileLegacyFormat, apiKey, apiSecret))
	if err != nil {
		return err
	}

	return nil
}

// setCredentialsFile creates/truncates the credential file: %home%/.veracode/veracode.yml
func setCredentialsFile(homeDir, apiKey, apiSecret string) error {
	file, err := os.Create(filepath.Join(homeDir, ".veracode", "veracode.yml"))
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = file.WriteString(fmt.Sprintf(credentialFileFormat, apiKey, apiSecret))
	if err != nil {
		return err
	}

	return nil
}

// credsResultMsg contains all of the content for the self credential update method.
type credsResultMsg struct {
	err    error
	resp   *veracode.Response
	result veracode.APICredentials
}

// CredentialsRefreshModel is a tea component that is used when running:
//
//	verapack credential refresh
type CredentialsRefreshModel struct {
	result    credsResultMsg
	homeDir   string
	client    *veracode.Client
	spinner   spinner.Model
	errs      []error
	state     int // 0:generating credentials,1:generating files,2:done (either successfully or not)
	filesDone int
}

// Init makes the initial call to the self generate credentials endpoint.
func (m CredentialsRefreshModel) Init() tea.Cmd {
	return tea.Batch(func() tea.Msg {
		var result credsResultMsg
		ctx := context.Background()

		result.result, result.resp, result.err = m.client.Identity.SelfGenerateCredentials(ctx)

		return result
	}, m.spinner.Tick)
}

func (m CredentialsRefreshModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case credsResultMsg:
		// The first msg that is sent will be credsResultMsg. (Not including tea messages)
		if msg.err != nil {
			m.errs = append(m.errs, msg.err)
			return m, tea.Quit
		}

		m.result = msg

		m.state = 1

		// start a batch of tasks to set the 2 credential files. They will return an anonymous struct with an error field.
		cmds = append(cmds, tea.Batch(
			func() tea.Msg {
				return struct{ err error }{err: setCredentialsFile(m.homeDir, msg.result.ApiId, msg.result.ApiSecret)}
			},
			func() tea.Msg {
				return struct{ err error }{err: setLegacyCredentialsFile(m.homeDir, msg.result.ApiId, msg.result.ApiSecret)}
			},
		))

	case struct{ err error }:
		// when the anonymous struct msg is received, it means that one of the creds update tasks have completed.
		m.filesDone += 1

		if msg.err != nil {
			m.errs = append(m.errs, msg.err)
		}

		if m.filesDone == 2 {
			m.state = 2 // set state to 2 indicating done. However, if there are errors in m.errs, error panel will be displayed.
			return m, tea.Quit
		}
	}

	var cmd tea.Cmd
	m.spinner, cmd = m.spinner.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

func (m CredentialsRefreshModel) View() string {
	var s string

	if len(m.errs) < 1 {
		style := lipgloss.NewStyle().Padding(0, 1, 1, 1).Margin(0, 0, 0, 2).Align(lipgloss.Center).Border(lipgloss.RoundedBorder()).BorderForeground(darkGray)
		switch m.state {
		case 0:
			// generating credentials
			s += style.Render(m.spinner.View() + "  Generating new credentials...")
		case 1:
			// generating files
			s += style.Render(m.spinner.View() + "  Updating Files...")
		case 2:
			// successfully done
			s += style.Render(fmt.Sprintf("%s\n\n%s%s", "Credentials have been successfully re-generated.", "Expiration Date: ", lightBlueForeground.Render(m.result.result.ExpirationTs.Local().Format("02 Jan 2006 15:04:05PM"))))
		}
	} else {
		s += renderErrors(m.errs...)
	}
	s += "\n"

	return s
}

func NewCredentialsRefreshModel(client *veracode.Client, homeDir string) CredentialsRefreshModel {
	return CredentialsRefreshModel{
		client:  client,
		homeDir: homeDir,
		spinner: spinner.New(defaultSpinnerOpts...),
	}
}

// CredentialsConfigureModel is a tea component that is used when running:
//
//	verapack credential configure
type CredentialsConfigureModel struct {
	CredentialsTask
	help      help.Model
	homeDir   string
	spinner   spinner.Model
	errs      []error
	state     int // 0:waiting for user input,2:done (either successfully or not)
	filesDone int
}

func NewCredentialsConfigureModel(credentialsTask CredentialsTask, homeDir string) CredentialsConfigureModel {
	return CredentialsConfigureModel{
		CredentialsTask: credentialsTask,
		help:            help.New(),
		spinner:         spinner.New(defaultSpinnerOpts...),
		homeDir:         homeDir,
	}
}

func (m CredentialsConfigureModel) Init() tea.Cmd {
	return m.CredentialsTask.Init()
}

func (m CredentialsConfigureModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC:
			return m, tea.Quit
		}
	case struct{ err error }:
		// when the anonymous struct msg is received, it means that one of the creds update tasks have completed.
		m.filesDone += 1

		if msg.err != nil {
			m.errs = append(m.errs, msg.err)
		}

		if m.filesDone == 2 {
			m.state = 1 // set state to 1 indicating done. However, if there are errors in m.errs, error panel will be displayed.
			return m, tea.Quit
		}
	}

	var cmd tea.Cmd

	if m.state == 0 {
		var model tea.Model
		model, cmd = m.CredentialsTask.Update(msg)
		m.CredentialsTask = model.(CredentialsTask)

		if m.state == 0 && m.CredentialsTask.isInputDone {
			return m, tea.Batch(
				func() tea.Msg {
					return struct{ err error }{err: setCredentialsFile(m.homeDir, m.apiKey, m.apiSecret)}
				},
				func() tea.Msg {
					return struct{ err error }{err: setLegacyCredentialsFile(m.homeDir, m.apiKey, m.apiSecret)}
				},
			)
		}
	}

	return m, cmd
}

func (m CredentialsConfigureModel) View() string {
	var s string

	if len(m.errs) < 1 {
		style := lipgloss.NewStyle().Padding(0, 1, 1, 1).Margin(0, 0, 0, 2).Border(lipgloss.RoundedBorder()).BorderForeground(darkGray)
		switch m.state {
		case 0:
			// waiting for user input
			s += style.Render(m.CredentialsTask.View()) + "\n" + m.help.ShortHelpView(append([]key.Binding{key.NewBinding(key.WithKeys("ctrl+c"), key.WithHelp("ctrl+c", "quit"))}, m.keys.ShortHelp()...))
		case 1:
			// done
			s += style.Align(lipgloss.Center).Render("Credentials have been successfully set.")
		}
	} else {
		s += renderErrors(m.errs...)
	}
	s += "\n"

	return s
}
