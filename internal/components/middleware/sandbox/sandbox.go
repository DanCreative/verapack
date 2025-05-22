package sandbox

import (
	"context"
	"fmt"
	"strings"

	"github.com/DanCreative/veracode-go/veracode"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type Option func(*Model)

type PostFunc func(model Model, size tea.WindowSizeMsg) (tea.Model, tea.Cmd)

type Styles struct {
	Border lipgloss.Style
	Counts lipgloss.Style
}

// Model is a tea component that will be used to display a loading screen while the
// sandboxes are created/fetched.
//
// Model does the following:
//   - Search for an application profile with provided name
//   - List the sandboxes for said ap
//   - If sandbox could not be found create it
//
// Model mutates the provided []Options and adds the found/created sandbox ids to it.
type Model struct {
	spinner        spinner.Model
	errRenderFunc  func(...error) string
	size           tea.WindowSizeMsg
	postFunc       PostFunc
	styles         Styles
	ctx            context.Context
	client         *veracode.Client
	errs           []error
	Applications   []SandboxOptions
	searchingCount *int
	creatingCount  *int
	createdCount   *int
	foundCount     *int
}

type SandboxOptions struct {
	AppName     string
	SandboxName string
	SandboxId   *int
	SandboxGuid *string
	AppGuid     *string
}

func (m Model) Init() tea.Cmd {
	cmds := make([]tea.Cmd, 0, len(m.Applications)+2)
	cmds = append(cmds, tea.ClearScreen, m.spinner.Tick)

	for k := range m.Applications {
		cmds = append(cmds, SearchApplication(m.Applications[k].AppName, k, m.client, m.ctx))
	}

	return tea.Batch(cmds...)
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.size = msg

	case applicationSearchMsg:
		// 1. this is the first msg that will be returned
		if msg.err != nil {
			m.errs = append(m.errs, msg.err)
			*m.searchingCount--

			if *m.createdCount+*m.foundCount+len(m.errs) == len(m.Applications) {
				// if all applications are accounted for, exit the model
				if m.postFunc != nil {
					return m.postFunc(m, m.size)
				}
				return m, tea.Quit
			}
			break
		}

		if *m.createdCount+*m.foundCount+len(m.errs) == len(m.Applications) {
			// if all applications are accounted for, exit the model
			if m.postFunc != nil {
				return m.postFunc(m, m.size)
			}
			return m, tea.Quit
		}

		// Mutate the []Options if the application is found
		*m.Applications[msg.appIndex].AppGuid = msg.appGuid

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
			*m.Applications[msg.appIndex].SandboxId = msg.sandboxId
			*m.Applications[msg.appIndex].SandboxGuid = msg.sandboxGuid

			if *m.createdCount+*m.foundCount+len(m.errs) == len(m.Applications) {
				// if all applications are accounted for, exit the model
				if m.postFunc != nil {
					return m.postFunc(m, m.size)
				}
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
		*m.Applications[msg.appIndex].SandboxId = msg.sandboxId
		*m.Applications[msg.appIndex].SandboxGuid = msg.sandboxGuid

		if *m.createdCount+*m.foundCount+len(m.errs) == len(m.Applications) {
			// if all applications are accounted for, exit the model
			if m.postFunc != nil {
				return m.postFunc(m, m.size)
			}
			return m, tea.Quit
		}
	}

	var cmd tea.Cmd
	m.spinner, cmd = m.spinner.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

func (m Model) View() string {
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
		s += m.styles.Counts.Render(fmt.Sprintf("   searching: %d", *m.searchingCount))

		if len(m.errs) > 0 || *m.foundCount > 0 || *m.createdCount > 0 || *m.creatingCount > 0 {
			s += "\n"
		}
	}

	if *m.creatingCount > 0 {
		s += m.styles.Counts.Render(fmt.Sprintf("   creating: %d", *m.creatingCount))

		if len(m.errs) > 0 || *m.foundCount > 0 || *m.createdCount > 0 {
			s += "\n"
		}
	}

	if *m.createdCount > 0 {
		s += m.styles.Counts.Render(fmt.Sprintf("   created: %d", *m.createdCount))

		if len(m.errs) > 0 || *m.foundCount > 0 {
			s += "\n"
		}
	}

	if *m.foundCount > 0 {
		s += m.styles.Counts.Render(fmt.Sprintf("   found: %d", *m.foundCount))

		if len(m.errs) > 0 {
			s += "\n"
		}
	}

	if errCount := len(m.errs); errCount > 0 {
		s += m.styles.Counts.Render(fmt.Sprintf("   errors: %d", errCount)) + "\n" + lipgloss.NewStyle().MarginLeft(3).Render(m.errRenderFunc(m.errs...))
	}

	return m.styles.Border.Render(s) + "\n"
}

func (m *Model) GetErrors() []error {
	return m.errs
}

func NewModel(applications []SandboxOptions, client *veracode.Client, ctx context.Context, options ...Option) Model {
	cc := 0
	ci := 0
	fc := 0
	sc := len(applications)

	m := Model{
		errs:           make([]error, 0, len(applications)),
		spinner:        spinner.New(),
		ctx:            ctx,
		client:         client,
		searchingCount: &sc,
		createdCount:   &cc,
		creatingCount:  &ci,
		foundCount:     &fc,
		Applications:   applications,
	}

	for _, opt := range options {
		opt(&m)
	}

	return m
}

func WithPostFunc(postFunc PostFunc) Option {
	return func(m *Model) {
		m.postFunc = postFunc
	}
}

func WithErrorRenderFunc(errRenderFunc func(...error) string) Option {
	return func(m *Model) {
		m.errRenderFunc = errRenderFunc
	}
}

func WithStyles(styles Styles) Option {
	return func(m *Model) {
		m.styles = styles
	}
}

func WithSpinner(opts ...spinner.Option) Option {
	return func(m *Model) {
		m.spinner = spinner.New(opts...)
	}
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

		for _, profile := range profiles {
			if strings.EqualFold(profile.Profile.Name, applicationName) {
				msg.appGuid = profile.Guid
				return msg
			}
		}

		msg.err = fmt.Errorf("no application profile found with name: '%s'", applicationName)
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
