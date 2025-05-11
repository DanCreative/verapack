package verapack

import (
	"context"
	"fmt"
	"strings"

	"github.com/DanCreative/veracode-go/veracode"
	"github.com/DanCreative/verapack/internal/components/reportcard"
	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// SandboxModel is a tea component that will be used for when the user runs a sandbox scan, but
// one or more applications do not have the sandbox_name field set.
type SandboxModel struct {
	selected     *int
	applications []*Options
	options      []string
	help         help.Model
	UpKey        key.Binding
	DownKey      key.Binding
	EnterKey     key.Binding
	QuitKey      key.Binding
}

func (m SandboxModel) Init() tea.Cmd {
	return nil
}

func (m SandboxModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, m.EnterKey):
			return m, tea.Quit
		case key.Matches(msg, m.QuitKey):
			*m.selected = 2
			return m, tea.Quit
		case key.Matches(msg, m.DownKey):
			m.nextOption()
		case key.Matches(msg, m.UpKey):
			m.prevOption()
		}
	}

	return m, nil
}

func (m SandboxModel) View() string {
	var apps string
	var options string

	for k, name := range m.options {
		switch name {
		case "cancel":
			if k == *m.selected {
				options += fmt.Sprintf("(%s)  %s\n", lightBlueForeground.Render("x"), lightBlueForeground.Render("Cancel the scan"))
			} else {
				options += "( )  Cancel the scan\n"
			}
		case "mixed":
			if k == *m.selected {
				options += fmt.Sprintf("(%s)  %s\n", lightBlueForeground.Render("x"), lightBlueForeground.Render("Run policy scans where the field was not provided"))
			} else {
				options += "( )  Run policy scans where the field was not provided\n"
			}
		case "sandboxonly":
			if k == *m.selected {
				options += fmt.Sprintf("(%s)  %s\n", lightBlueForeground.Render("x"), lightBlueForeground.Render("Only scan the applications with the provided field"))
			} else {
				options += "( )  Only scan the applications with the provided field\n"
			}
		}
	}

	for k := range m.applications {
		apps += darkGrayForeground.Render("\n\t•  " + m.applications[k].AppName)
	}

	s := fmt.Sprintf("Below applications are missing field: %s\n%s\n\nWhat would you like to do?\n\n%s",
		lipgloss.NewStyle().Bold(true).Inline(true).Render("sandbox_name"),
		apps,
		options,
	)

	return lipgloss.NewStyle().
		Padding(0, 1, 1, 1).
		Margin(0, 0, 0, 2).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(darkGray).Render(s) + "\n" + m.help.ShortHelpView([]key.Binding{m.QuitKey, m.UpKey, m.DownKey, m.EnterKey})
}

func (m *SandboxModel) nextOption() {
	if m.options != nil {
		*m.selected = (*m.selected + 1) % len(m.options)
	}
}

func (m *SandboxModel) prevOption() {
	if m.options != nil {
		*m.selected--
		// Wrap around
		if *m.selected < 0 {
			*m.selected = len(m.options) - 1
		}
	}
}

func (m *SandboxModel) GetSelection() int {
	return *m.selected
}

func NewSandboxModel(applications []*Options) SandboxModel {
	s := 0
	return SandboxModel{
		selected: &s,
		options: []string{
			"sandboxonly",
			"mixed",
			"cancel",
		},
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
		applications: applications,
	}
}

// HandleSandboxNotProvided handles the case where the user does not provide the sandbox_name
// for 1 or more of the applications in the config file.
//
// It prompts the user to decide how to proceed with the erroneous applications. The user can
// either:
//
//   - Cancel the scan
//   - Run only the applications that are correct
//   - Run sandbox scans for the correct apps and policy scans for the incorrect apps.
//
// The function returns an int which will be 1 if the user indicated that they wish to cancel
// the run, the modified list of applications and an error if one occurred.
func HandleSandboxNotProvided(applications []Options, defaultScanType ScanType) (int, []Options, error) {
	badApps := make([]*Options, 0)

	for k := range applications {
		if applications[k].SandboxName == "" {
			badApps = append(badApps, &applications[k])
		} else {
			applications[k].ScanType = defaultScanType
		}
	}

	// If there are no erroneous apps, return the full application list.
	if len(badApps) == 0 {
		return 0, applications, nil
	}

	m := NewSandboxModel(badApps)

	p := tea.NewProgram(m)

	if _, err := p.Run(); err != nil {
		return 0, nil, err
	}

	switch m.GetSelection() {
	case 2:
		return 1, nil, nil

	case 1:
		for k := range badApps {
			badApps[k].ScanType = ScanTypePolicy
		}

		return 0, applications, nil

	case 0:
		newApplications := make([]Options, 0, len(applications)-len(badApps))

		for j := range applications {
			var isBad bool

			for k := range badApps {
				if badApps[k].AppName == applications[j].AppName {
					isBad = true
				}
			}
			if !isBad {
				newApplications = append(newApplications, applications[j])
			}
		}

		applications = newApplications

		return 0, newApplications, nil
	}

	return 0, nil, nil
}

func renderBodyText(applications []*Options) string {
	var apps string
	for k := range applications {
		apps += darkGrayForeground.Render("\n\t•  " + applications[k].AppName)
	}

	return fmt.Sprintf("Below applications are missing field: %s\n%s\n\nWhat would you like to do?",
		lipgloss.NewStyle().Bold(true).Inline(true).Render("sandbox_name"),
		apps,
	)
}

// SandboxLoadingModel is a tea component that will be used to display a loading screen while the
// sandboxes are created/fetched.
//
// SandboxLoadingModel does the following:
//   - Search for an application profile with provided name
//   - List the sandboxes for said ap
//   - If sandbox could not be found create it
//
// SandboxLoadingModel mutates the provided []Options and adds the found/created sandbox ids to it.
type SandboxLoadingModel struct {
	spinner        spinner.Model
	ctx            context.Context
	client         *veracode.Client
	errs           []error
	Applications   []*Options
	searchingCount *int
	creatingCount  *int
	createdCount   *int
	foundCount     *int
}

func (m SandboxLoadingModel) Init() tea.Cmd {
	cmds := make([]tea.Cmd, 0, len(m.Applications)+2)
	cmds = append(cmds, tea.ClearScreen, m.spinner.Tick)

	for k := range m.Applications {
		cmds = append(cmds, SearchApplication(m.Applications[k].AppName, k, m.client, m.ctx))
	}

	return tea.Batch(cmds...)
}

func (m SandboxLoadingModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case applicationSearchMsg:
		// 1. this is the first msg that will be returned
		if msg.err != nil {
			m.errs = append(m.errs, msg.err)
			*m.searchingCount--

			if *m.createdCount+*m.foundCount+len(m.errs) == len(m.Applications) {
				// if all applications are accounted for, exit the model
				return m, tea.Quit
			}
			break
		}

		if *m.createdCount+*m.foundCount+len(m.errs) == len(m.Applications) {
			// if all applications are accounted for, exit the model
			return m, tea.Quit
		}

		// application was found, therefore perform the next step which is searching for an existing sandbox.
		cmds = append(cmds, SearchSandbox(msg.appGuid, m.Applications[msg.appIndex].SandboxName, msg.appIndex, m.client, msg.ctx))

	case sandboxSearchMsg:
		// 2. this is the second msg that will be returned
		*m.searchingCount--

		if msg.err != nil {
			m.errs = append(m.errs, msg.err)
			break
		}

		if msg.sandboxFound {
			*m.foundCount++

			// Mutate the []Options if a sandbox is found
			m.Applications[msg.appIndex].SandboxId = msg.sandboxId
			m.Applications[msg.appIndex].SandboxGuid = msg.sandboxGuid

			if *m.createdCount+*m.foundCount+len(m.errs) == len(m.Applications) {
				// if all applications are accounted for, exit the model
				return m, tea.Quit
			}

			break
		}

		*m.creatingCount++
		cmds = append(cmds, CreateSandbox(msg.appGuid, msg.sandboxName, msg.appIndex, m.client, msg.ctx))

	case sandboxCreateMsg:
		// 3. this is the third msg that will be returned
		*m.creatingCount--

		if msg.err != nil {
			m.errs = append(m.errs, msg.err)
			break
		}

		*m.createdCount++

		// Mutate the []Options if a sandbox is created
		m.Applications[msg.appIndex].SandboxId = msg.sandboxId
		m.Applications[msg.appIndex].SandboxGuid = msg.sandboxGuid

		if *m.createdCount+*m.foundCount+len(m.errs) == len(m.Applications) {
			// if all applications are accounted for, exit the model
			return m, tea.Quit
		}
	}

	var cmd tea.Cmd
	m.spinner, cmd = m.spinner.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

func (m SandboxLoadingModel) View() string {
	var s string

	if *m.createdCount+*m.foundCount+len(m.errs) == len(m.Applications) {
		if len(m.errs) > 0 {
			s = "✗  Finished with errors\n"
		} else {
			s = "✓  Successfully completed\n"
		}
	} else {
		s = m.spinner.View() + "  Fetching/Creating sandboxes..." + "\n"
	}

	if *m.searchingCount > 0 {
		s += darkGrayForeground.Render(fmt.Sprintf("   searching: %d", *m.searchingCount))

		if len(m.errs) > 0 || *m.foundCount > 0 || *m.createdCount > 0 || *m.creatingCount > 0 {
			s += "\n"
		}
	}

	if *m.creatingCount > 0 {
		s += darkGrayForeground.Render(fmt.Sprintf("   creating: %d", *m.creatingCount))

		if len(m.errs) > 0 || *m.foundCount > 0 || *m.createdCount > 0 {
			s += "\n"
		}
	}

	if *m.createdCount > 0 {
		s += darkGrayForeground.Render(fmt.Sprintf("   created: %d", *m.createdCount))

		if len(m.errs) > 0 || *m.foundCount > 0 {
			s += "\n"
		}
	}

	if *m.foundCount > 0 {
		s += darkGrayForeground.Render(fmt.Sprintf("   found: %d", *m.foundCount))

		if len(m.errs) > 0 {
			s += "\n"
		}
	}

	if errCount := len(m.errs); errCount > 0 {
		s += darkGrayForeground.Render(fmt.Sprintf("   errors: %d", errCount)) + "\n" + lipgloss.NewStyle().MarginLeft(3).Render(rawRenderErrors(m.errs...))
	}

	return lipgloss.NewStyle().
		Padding(0, 1, 1, 1).
		Margin(0, 0, 0, 2).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(darkGray).Render(s) + "\n"
}

func NewSandboxLoadingModel(client *veracode.Client, ctx context.Context, applications []Options) SandboxLoadingModel {
	cc := 0
	ci := 0
	fc := 0

	sandboxApplications := make([]*Options, 0, len(applications))

	for k := range applications {
		if st := applications[k].ScanType; st == ScanTypeSandbox || st == ScanTypePromote {
			sandboxApplications = append(sandboxApplications, &applications[k])
		}
	}

	sc := len(sandboxApplications)

	return SandboxLoadingModel{
		client:         client,
		ctx:            ctx,
		Applications:   sandboxApplications,
		errs:           make([]error, 0, len(applications)),
		spinner:        spinner.New(defaultSpinnerOpts...),
		searchingCount: &sc,
		createdCount:   &cc,
		creatingCount:  &ci,
		foundCount:     &fc,
	}
}

func promoteSandboxes(client *veracode.Client, ctx context.Context, app Options, appId int, reporter reporter) {
	// Find application profile with name
	profiles, _, err := client.Application.ListApplications(ctx, veracode.ListApplicationOptions{Name: app.AppName})
	if err != nil {
		reporter.Send(reportcard.TaskResultMsg{
			Status:  reportcard.Failure,
			Output:  err.Error(),
			Index:   appId,
			IsFatal: true,
		})
		return
	}

	if len(profiles) == 0 {
		// Could not find an application profile with name
		reporter.Send(reportcard.TaskResultMsg{
			Status:  reportcard.Failure,
			Output:  fmt.Sprintf("no application profile found with name: '%s'", app.AppName),
			Index:   appId,
			IsFatal: true,
		})
		return
	}

	if len(profiles) > 1 {
		reporter.Send(reportcard.TaskResultMsg{
			Status:  reportcard.Failure,
			Output:  fmt.Sprintf("more than 1 application profile found with name: '%s'", app.AppName),
			Index:   appId,
			IsFatal: true,
		})
		return
	}

	sandbox, _, err := client.Sandbox.GetSandbox(ctx, profiles[0].Guid, app.SandboxGuid)
	if err != nil {
		reporter.Send(reportcard.TaskResultMsg{
			Status:  reportcard.Failure,
			Output:  err.Error(),
			Index:   appId,
			IsFatal: true,
		})
		return
	}

	_, _, err = client.Sandbox.PromoteSandbox(ctx, profiles[0].Guid, sandbox.Guid, true)
	if err != nil {
		reporter.Send(reportcard.TaskResultMsg{
			Status:  reportcard.Failure,
			Output:  err.Error(),
			Index:   appId,
			IsFatal: true,
		})
		return
	}

	reporter.Send(reportcard.TaskResultMsg{
		Status: reportcard.Success,
		Index:  appId,
		Output: "",
	})
}

// Tea Command builders
type applicationSearchMsg struct {
	err      error
	appIndex int
	appGuid  string
	name     string
	ctx      context.Context
}

func SearchApplication(applicationName string, appIndex int, client *veracode.Client, ctx context.Context) tea.Cmd {
	msg := applicationSearchMsg{
		name:     applicationName,
		ctx:      ctx,
		appIndex: appIndex,
	}

	return func() tea.Msg {
		// Find application profile with name
		profiles, _, err := client.Application.ListApplications(ctx, veracode.ListApplicationOptions{Name: applicationName})
		if err != nil {
			msg.err = err
			return msg
		}

		if len(profiles) == 0 {
			// Could not find an application profile with name
			msg.err = fmt.Errorf("no application profile found with name: '%s'", applicationName)
			return msg
		}

		if len(profiles) > 1 {
			msg.err = fmt.Errorf("more than 1 application profile found with name: '%s'", applicationName)
			return msg
		}

		msg.appGuid = profiles[0].Guid
		return msg
	}
}

type sandboxSearchMsg struct {
	err          error
	appGuid      string
	sandboxName  string
	sandboxId    int
	sandboxGuid  string
	sandboxFound bool
	appIndex     int
	ctx          context.Context
}

func SearchSandbox(appGuid string, sandboxName string, appIndex int, client *veracode.Client, ctx context.Context) tea.Cmd {
	msg := sandboxSearchMsg{
		appGuid:     appGuid,
		appIndex:    appIndex,
		sandboxName: sandboxName,
		ctx:         ctx,
	}

	return func() tea.Msg {
		// Get a list of sandboxes for the application profile
		sandboxes, _, err := client.Sandbox.ListSandboxes(ctx, appGuid, veracode.PageOptions{})
		if err != nil {
			msg.err = err
			return err
		}

		// If a sandbox in the list matches the provided sandbox name, set sandboxGuid to its GUID value
		for _, sandbox := range sandboxes {
			if strings.EqualFold(sandbox.Name, sandboxName) {
				msg.sandboxId = sandbox.Id
				msg.sandboxGuid = sandbox.Guid
				break
			}
		}

		// sandboxGuid will be empty if an existing sandbox could not be found.
		if msg.sandboxId != 0 {
			msg.sandboxFound = true
		}

		return msg
	}
}

type sandboxCreateMsg struct {
	err         error
	appGuid     string
	sandboxName string
	sandboxId   int
	sandboxGuid string
	appIndex    int
	ctx         context.Context
}

func CreateSandbox(appGuid string, sandboxName string, appIndex int, client *veracode.Client, ctx context.Context) tea.Cmd {
	msg := sandboxCreateMsg{
		appGuid:     appGuid,
		appIndex:    appIndex,
		sandboxName: sandboxName,
		ctx:         ctx,
	}

	return func() tea.Msg {
		sandbox, _, err := client.Sandbox.CreateSandbox(ctx, appGuid, veracode.CreateSandbox{Name: sandboxName, AutoCreate: true})
		if err != nil {
			msg.err = err
			return msg
		}

		msg.sandboxId = sandbox.Id
		msg.sandboxGuid = sandbox.Guid
		return msg
	}
}
