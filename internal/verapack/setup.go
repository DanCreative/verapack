package verapack

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/DanCreative/verapack/internal/components/multistagesetup"
	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var _ multistagesetup.TeaTasker = SimpleTask{}
var _ multistagesetup.TeaTasker = CredentialsTask{}
var _ multistagesetup.TeaTasker = PrerequisiteTask{}

// ===================================================================== TeaTaskers =====================================================================================

type SimpleTask struct {
	f      func(values map[string]any) tea.Cmd
	values map[string]any
	// f tea.Cmd // tea function that will be run
}

func (s SimpleTask) GetHelp() help.KeyMap {
	return nil
}

func (s SimpleTask) Init() tea.Cmd {
	return s.f(s.values)
}

func (s SimpleTask) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	return s, nil
}

func (s SimpleTask) View() string {
	return ""
}

func (s SimpleTask) NewWithValues(values map[string]any) multistagesetup.TeaTasker {
	s.values = values
	return s
}

func NewSimpleTask(f func(values map[string]any) tea.Cmd) SimpleTask {
	return SimpleTask{
		f: f,
	}
}

type PrerequisiteTaskResult struct {
	warnings []string
}

type PrerequisiteTask struct {
	f              tea.Cmd
	result         PrerequisiteTaskResult
	acknowledgeKey key.Binding
}

func (s PrerequisiteTask) GetHelp() help.KeyMap {
	return s
}

func (s PrerequisiteTask) Init() tea.Cmd {
	return s.f
}

func (s PrerequisiteTask) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, s.acknowledgeKey):
			// This case will only be possible if there are warnings.
			return s, func() tea.Msg { return multistagesetup.NewWarningTaskResult("", nil) }
		}
	case PrerequisiteTaskResult:
		if len(msg.warnings) < 1 {
			return s, func() tea.Msg { return multistagesetup.NewSuccessfulTaskResult("", nil) }
		}
		s.result = msg
	}

	return s, nil
}

func (s PrerequisiteTask) View() string {
	var r string

	symbolStyle := lipgloss.NewStyle().Width(5)

	for _, warning := range s.result.warnings {
		r += symbolStyle.Foreground(orange).Render("⚠") + warning + "\n"
	}

	return r
}

func (s PrerequisiteTask) NewWithValues(values map[string]any) multistagesetup.TeaTasker {
	return s
}

func (s PrerequisiteTask) ShortHelp() []key.Binding {
	return []key.Binding{s.acknowledgeKey}
}

func (s PrerequisiteTask) FullHelp() [][]key.Binding {
	return nil
}

func NewPrerequisiteTask(f tea.Cmd) PrerequisiteTask {
	return PrerequisiteTask{
		f: f,
		acknowledgeKey: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "acknowledge"),
		),
	}
}

// CredentialsTask is a tea component that is used to prompt the user for their
// api key and secret.
type CredentialsTask struct {
	inputs            []textinput.Model
	keys              CredentialsFormKeyMap
	focused           int
	isInputDone       bool
	apiKey, apiSecret string
	loadCredFunc      func() (string, string, error)
}

func (m *CredentialsTask) updateHelp() {
	if len(m.inputs)-1 == m.focused {
		m.keys.Submit.SetEnabled(true)
		m.keys.Next.SetEnabled(false)
	} else {
		m.keys.Submit.SetEnabled(false)
		m.keys.Next.SetEnabled(true)
	}
}

func (m *CredentialsTask) nextInput() {
	if m.inputs != nil {
		m.inputs[m.focused].Blur()
		m.focused = (m.focused + 1) % len(m.inputs)
		m.inputs[m.focused].Focus()

		m.updateHelp()
	}
}

func (m *CredentialsTask) prevInput() {
	if m.inputs != nil {
		m.inputs[m.focused].Blur()

		m.focused--
		// Wrap around
		if m.focused < 0 {
			m.focused = len(m.inputs) - 1
		}

		m.inputs[m.focused].Focus()

		m.updateHelp()
	}
}

func (m CredentialsTask) Init() tea.Cmd {
	return tea.Batch(textinput.Blink, func() tea.Msg {
		apiKey, apiSecret, err := m.loadCredFunc()
		if err == nil {
			m.apiKey, m.apiSecret = apiKey, apiSecret
			return multistagesetup.NewSkippedTaskResult("already setup", map[string]any{"apiKey": apiKey, "apiSecret": apiSecret})
		}

		return nil
	})
}

func (m CredentialsTask) NewWithValues(values map[string]any) multistagesetup.TeaTasker {
	return m
}

func (m CredentialsTask) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var pasted bool

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, m.keys.Submit):
			if m.focused == len(m.inputs)-1 {
				m.isInputDone = true
				m.apiKey = m.inputs[0].Value()
				m.apiSecret = m.inputs[1].Value()
				return m, func() tea.Msg {
					return multistagesetup.NewSuccessfulTaskResult("", map[string]any{"apiKey": m.apiKey, "apiSecret": m.apiSecret})
				}
			} else {
				m.nextInput()
			}
		case key.Matches(msg, m.keys.Next):
			m.nextInput()
		case key.Matches(msg, m.keys.Prev):
			m.prevInput()
		case msg.Type == tea.KeyCtrlV:
			pasted = true
		}
	}

	cmds := make([]tea.Cmd, len(m.inputs))

	for k := range m.inputs {
		m.inputs[k], cmds[k] = m.inputs[k].Update(msg)
	}

	if pasted && m.focused != len(m.inputs)-1 {
		return m, tea.Sequence(tea.Batch(cmds...), func() tea.Msg { return tea.KeyMsg{Type: tea.KeyEnter} })
	} else {
		return m, tea.Batch(cmds...)
	}
}

func (m CredentialsTask) View() string {
	if m.isInputDone {
		return ""
	}
	s := fmt.Sprintf(`%s
%s
	
%s
%s%s
%s%s`,
		"If you do not have a Veracode API ID and Secret Key, navigate to one of below URLs (based on your region) to generate your API credentials:",
		lightBlueForeground.Render("•")+"\thttps://analysiscenter.veracode.eu/auth/index.jsp#APICredentialsGenerator\n"+lightBlueForeground.Render("•")+"\thttps://analysiscenter.veracode.com/auth/index.jsp#APICredentialsGenerator\n"+lightBlueForeground.Render("•")+"\thttps://analysiscenter.veracode.us/auth/index.jsp#APICredentialsGenerator\n",
		"Once you have generated your API credentials, please enter/paste your ID and secret below:\n",
		lightBlueForeground.Width(12).Render("API ID: "),
		m.inputs[0].View(),
		lightBlueForeground.Width(12).Render("API Secret: "),
		m.inputs[1].View())

	return s
}

func (m CredentialsTask) GetHelp() help.KeyMap {
	return m.keys
}

type CredentialsFormKeyMap struct {
	Next   key.Binding
	Prev   key.Binding
	Submit key.Binding
}

func (k CredentialsFormKeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Next, k.Submit, k.Prev}
}

func (k CredentialsFormKeyMap) FullHelp() [][]key.Binding {
	return nil
}

func NewCredentialsTask(loadCredFunc func() (string, string, error)) CredentialsTask {
	inputs := make([]textinput.Model, 2)
	inputs[0] = textinput.New()
	inputs[0].Focus()
	inputs[0].Prompt = ""

	inputs[1] = textinput.New()
	inputs[1].EchoCharacter = '•'
	inputs[1].EchoMode = textinput.EchoPassword
	inputs[1].Prompt = ""

	return CredentialsTask{
		inputs:       inputs,
		loadCredFunc: loadCredFunc,
		keys: CredentialsFormKeyMap{
			Next: key.NewBinding(
				key.WithHelp("tab/enter", "next"),
				key.WithKeys(tea.KeyTab.String(), tea.KeyEnter.String()),
			),
			Prev: key.NewBinding(
				key.WithHelp(tea.KeyShiftTab.String(), "prev"),
				key.WithKeys(tea.KeyShiftTab.String()),
			),
			Submit: key.NewBinding(
				key.WithDisabled(),
				key.WithHelp("enter", "Submit"),
				key.WithKeys(tea.KeyEnter.String()),
			),
		},
	}
}

// ===================================================================== TeaTaskers =====================================================================================

// Tasks
func SetupConfig(homeDir, appDir string) multistagesetup.SetupTask {
	return multistagesetup.NewSetupTask("Set up initial config template", NewSimpleTask(
		func(values map[string]any) tea.Cmd {
			return func() tea.Msg {
				var err error

				_, err = os.Stat(filepath.Join(appDir, "config.yaml"))
				if err == nil {
					return multistagesetup.NewSkippedTaskResult("already setup", nil)
				}

				if err = os.MkdirAll(appDir, 0600); err != nil {
					return multistagesetup.NewFailedTaskResult("", err, nil)
				}

				file, err := os.Create(filepath.Join(appDir, "config.yaml"))
				if err != nil {
					return multistagesetup.NewFailedTaskResult("", err, nil)
				}

				defer file.Close()

				_, err = io.Copy(file, bytes.NewReader(configFileBytes))
				if err != nil {
					return multistagesetup.NewFailedTaskResult("", err, nil)
				}

				return multistagesetup.NewSuccessfulTaskResult("", nil)
			}
		},
	))
}

func SetupCredentialsUserPrompt(loadCredFunc func() (string, string, error)) multistagesetup.SetupTask {
	return multistagesetup.NewSetupTask("User generate and enter credentials", NewCredentialsTask(loadCredFunc))
}

func SetupCredentialsFile(homeDir string) multistagesetup.SetupTask {
	return multistagesetup.NewSetupTask("Create credential file", NewSimpleTask(func(values map[string]any) tea.Cmd {
		return func() tea.Msg {
			var err error
			if _, err = os.Stat(filepath.Join(homeDir, ".veracode", "veracode.yml")); err == nil {
				return multistagesetup.NewSkippedTaskResult("already setup", nil)
			}

			apiKey, okKey := values["apiKey"].(string)
			apiSecret, okSecret := values["apiSecret"].(string)

			if !okKey || !okSecret {
				return multistagesetup.NewFailedTaskResult("", errors.New("api key and/or secret not set"), nil)
			}

			err = setCredentialsFile(homeDir, apiKey, apiSecret)
			if err != nil {
				return multistagesetup.NewFailedTaskResult("", err, nil)
			}

			return multistagesetup.NewSuccessfulTaskResult("", nil)
		}
	}))
}

func SetupCredentialsFileLegacy(homeDir string) multistagesetup.SetupTask {
	return multistagesetup.NewSetupTask("Create legacy credential file", NewSimpleTask(func(values map[string]any) tea.Cmd {
		return func() tea.Msg {
			var err error
			if _, err = os.Stat(filepath.Join(homeDir, ".veracode", "credentials")); err == nil {
				return multistagesetup.NewSkippedTaskResult("already setup", nil)
			}

			apiKey, okKey := values["apiKey"].(string)
			apiSecret, okSecret := values["apiSecret"].(string)

			if !okKey || !okSecret {
				return multistagesetup.NewFailedTaskResult("", errors.New("api key and/or secret not set"), nil)
			}

			err = setLegacyCredentialsFile(homeDir, apiKey, apiSecret)
			if err != nil {
				return multistagesetup.NewFailedTaskResult("", err, nil)
			}

			return multistagesetup.NewSuccessfulTaskResult("", nil)
		}
	}))
}

func SetupInstallDependencyPackager() multistagesetup.SetupTask {
	return multistagesetup.NewSetupTask("Install Veracode CLI", NewSimpleTask(func(values map[string]any) tea.Cmd {
		return func() tea.Msg {
			packagerPath := getPackagerLocation()

			var err error

			if _, err = os.Stat(packagerPath); err == nil {
				localVersion := GetLocalVersion(filepath.Join(packagerPath, "VERSION"))
				return multistagesetup.NewSkippedTaskResult("already installed version: "+localVersion, nil)
			}

			version, err := InstallPackager(false, packagerPath)
			if err != nil {
				return multistagesetup.NewFailedTaskResult("", err, nil)
			}

			return multistagesetup.NewSuccessfulTaskResult("successfully installed version: "+version, nil)
		}
	}))
}

func UpdateDependencyPackager() multistagesetup.SetupTask {
	return multistagesetup.NewSetupTask("Update Veracode CLI", NewSimpleTask(func(values map[string]any) tea.Cmd {
		return func() tea.Msg {
			packagerPath := getPackagerLocation()
			packagerCurrentVersion := GetLocalVersion(filepath.Join(getPackagerLocation(), "VERSION"))

			jar, _ := cookiejar.New(&cookiejar.Options{})
			client := &http.Client{
				Jar: jar,
			}
			baseURL, _ := url.Parse("https://tools.veracode.com/veracode-cli")

			fileVersion, _ := GetLatestPackagerVersion(client, baseURL)

			if fileVersion == packagerCurrentVersion {
				return multistagesetup.NewSkippedTaskResult("already on the latest version: "+fileVersion, nil)
			}

			version, err := InstallPackager(false, packagerPath)
			if err != nil {
				return multistagesetup.NewFailedTaskResult("", err, nil)
			}

			return multistagesetup.NewSuccessfulTaskResult(fmt.Sprintf("successfully updated: %s -> %s", packagerCurrentVersion, version), nil)
		}
	}))
}

func SetupInstallDependencyWrapper() multistagesetup.SetupTask {
	return multistagesetup.NewSetupTask("Install Veracode Uploader", NewSimpleTask(func(values map[string]any) tea.Cmd {
		return func() tea.Msg {
			wrapperPath := getWrapperLocation()

			var err error

			if _, err = os.Stat(filepath.Join(wrapperPath, "VeracodeJavaAPI.jar")); err == nil {
				localVersion := GetLocalVersion(filepath.Join(wrapperPath, "VERSION"))
				return multistagesetup.NewSkippedTaskResult("already installed version: "+localVersion, nil)
			}

			version, err := InstallUploader(wrapperPath)
			if err != nil {
				return multistagesetup.NewFailedTaskResult("", err, nil)
			}

			return multistagesetup.NewSuccessfulTaskResult("successfully installed version: "+version, nil)
		}
	}))
}

func UpdateDependencyWrapper() multistagesetup.SetupTask {
	return multistagesetup.NewSetupTask("Update Veracode Uploader", NewSimpleTask(func(values map[string]any) tea.Cmd {
		return func() tea.Msg {
			wrapperPath := getWrapperLocation()
			wrapperCurrentVersion := GetLocalVersion(filepath.Join(wrapperPath, "VERSION"))

			jar, _ := cookiejar.New(&cookiejar.Options{})
			client := &http.Client{
				Jar: jar,
			}

			latestVersion, _ := GetLatestUploaderVersion(client)

			if wrapperCurrentVersion == latestVersion {
				return multistagesetup.NewSkippedTaskResult("already on the latest version: "+wrapperCurrentVersion, nil)
			}

			version, err := InstallUploader(wrapperPath)
			if err != nil {
				return multistagesetup.NewFailedTaskResult("", err, nil)
			}

			return multistagesetup.NewSuccessfulTaskResult(fmt.Sprintf("successfully updated: %s -> %s", wrapperCurrentVersion, version), nil)
		}
	}))
}

func SetupInstallScaAgent() multistagesetup.SetupTask {
	return multistagesetup.NewSetupTask("Install SCA Agent", NewSimpleTask(func(values map[string]any) tea.Cmd {
		return func() tea.Msg {
			packagerPath := getPackagerLocation()

			if err := InstallScaAgent(packagerPath); err != nil {
				return multistagesetup.NewFailedTaskResult("", err, nil)
			}

			return multistagesetup.NewSuccessfulTaskResult("successfully installed", nil)
		}
	}))
}

func Prerequisites() multistagesetup.SetupTask {
	return multistagesetup.NewSetupTask("Check prerequisites", NewPrerequisiteTask(
		func() tea.Msg {
			p := PrerequisiteTaskResult{
				warnings: make([]string, 0, 2),
			}

			_, err := exec.LookPath("java")
			if err != nil {
				p.warnings = append(p.warnings, "Java was not found on the path. You can either install Java version 8, 11 or 17.")
			}

			_, err = exec.LookPath("git")
			if err != nil {
				p.warnings = append(p.warnings, "git was not found on the path.")
			}

			return p
		},
	))
}
