package verapack

import (
	"fmt"
	"math"
	"reflect"
	"strconv"
	"strings"

	"github.com/DanCreative/verapack/internal/components/checkbox"
	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const (
	// appHeight is the height in lines of the visual model.
	appHeight = 20
	// inputs in the editor are paginated and inputsPerPage
	// sets the number of inputs per page.
	inputsPerPage = 3

	inputTypeTextArea = iota
	inputTypeTextInput
	inputTypeBool
)

var helpText = map[string]string{
	"app_name":              "Name of the application profile.",
	"create_profile":        "Create a application profile if the one provided in AppName does not exist.",
	"artefact_paths":        "Path(s) of the folders and files you want to upload to Veracode for scanning.",
	"version":               "Name or version of the build that you want to scan.",
	"scan_timeout":          "Number of minutes to wait for the scan to complete and pass policy.",
	"scan_polling_interval": "Interval, in seconds, to poll for the status of a running scan. Value range is 30 to 120 (two minutes). Default is 120.",
	"verbose":               "Displays detailed output.",
	"auto_cleanup":          "Automatically remove packaged output after scan completes.",
	"package_source":        "Location of the source to package based on the target --type. If the target is directory, \nenter the path to a local directory. If the target is repo, enter the URL to a Git version control system.",
	"trust":                 "Acknowledges that the source project is a trusted source. Required the first time you package a project.",
	"type":                  "Specifies the target type you want to package. Values are repo or directory. Default is directory.",
}

var (
	// See below diagram of the interface layout:
	// |---------| ----------------------|
	// |app list | editor				 |
	// |         | ----------------------|
	// |		 | |editorFieldsBlock   ||
	// |		 | ----------------------|
	// |		 |  page display         |
	// |---------| ----------------------|

	// applistStyle is a lipgloss.Style that is used to create the application list block.
	//
	// Its height is set to the appHeight.
	applistStyle = lipgloss.NewStyle().
			Width(30).
			Height(appHeight).
			Padding(0, 1, 1, 1).
			Border(lipgloss.RoundedBorder())

	// editorStyle is a lipgloss.Style that is used to create the editor block.
	//
	// Its height is set to the appHeight.
	editorStyle = lipgloss.NewStyle().
			Width(100).
			Height(appHeight).
			Padding(0, 1, 1, 1).
			Border(lipgloss.RoundedBorder(), true, true, true, false)

	// editorFieldsBlock is a lipgloss.Style that contains all of the fields in the editor.
	// It allows me to easily horizontally center the pages display at the bottom of the
	// editor block.
	//
	// Its width is the same as its parent and its width is the same as its parent minus padding, borders and heading height.
	editorFieldsBlock = lipgloss.NewStyle().
				Width(editorStyle.GetWidth()).
				Height(editorStyle.GetHeight() - editorStyle.GetVerticalPadding() - editorStyle.GetVerticalBorderSize() - 1)

	// headerStyle is a lipgloss.Style that is used for the primary headers.
	headerStyle = lipgloss.NewStyle().
			Padding(0, 0, 1, 0).
			AlignHorizontal(lipgloss.Center).
			Underline(true)

	// itemStyle is a lipgloss.Style that is used for the items in the application list.
	itemStyle = lipgloss.NewStyle().
			Padding(0, 1).
			Border(lipgloss.NormalBorder(), false, false, false, true).
			BorderForeground(darkGray)

	// selectedItemStyle is a lipgloss.Style that is used to show which item is being edited.
	selectedItemStyle = lipgloss.NewStyle().
				Padding(0, 1).
				Border(lipgloss.NormalBorder(), false, false, false, true).
				BorderForeground(darkBlue).
				Foreground(darkBlue)

	// selectedFocusedItemStyle is a lipgloss.Style that is used for the items in the application list,
	// when they are focussed.
	selectedFocusedItemStyle = lipgloss.NewStyle().
					Padding(0, 1).
					Border(lipgloss.NormalBorder(), false, false, false, true).
					BorderForeground(lightBlue).
					Foreground(lightBlue)
)

type configureTask struct {
	config Config
	state  int // state can be one of the following: 0: application list, 1: editor

	// app list

	selectedApp int

	// editor

	inputs       []input
	focusedInput int
}

func NewConfigureTask() configureTask {
	var c configureTask

	// Setting c.focusedInput to -1 removes all focus in the editor.
	c.focusedInput = -1
	c.createInputs()

	// Can only set the input values if there are applications available.
	if len(c.config.Applications) != 0 {
		c.setInputs(false)
	}
	return c
}

func (m configureTask) Init() tea.Cmd {
	return tea.Batch(textarea.Blink, textinput.Blink)
}

func (m configureTask) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	if m.state == 0 {
		// state = application list
		switch msg := msg.(type) {
		case tea.KeyMsg:
			switch msg.Type {
			case tea.KeyCtrlN:
				cmds = append(cmds, m.newApplication())
			case tea.KeyTab:
				m.prevApplication()
			case tea.KeyShiftTab:
				m.nextApplication()
			case tea.KeyEnter:
				cmds = append(cmds, m.setState(1))
			}
		}
	} else {
		// state = editor
		switch msg := msg.(type) {
		case tea.KeyMsg:
			switch msg.Type {
			case tea.KeyTab:
				cmds = append(cmds, m.nextInput())
			case tea.KeyShiftTab:
				cmds = append(cmds, m.prevInput())
			// case tea.KeyEnter:
			// 	// The TextArea uses enter to go to next line
			// 	// This ignores the enter key if it the focused field is a TextArea
			// 	if _, ok := m.inputs[m.focusedInput].inputer.(*textarea.Model); !ok {
			// 		cmds = append(cmds, m.nextInput())
			// 	}
			case tea.KeyCtrlS:
				cmds = append(cmds, m.setState(0))
			}
		}
	}

	for i := range m.inputs {
		cmds = append(cmds, m.inputs[i].Update(msg))
	}

	return m, tea.Batch(cmds...)
}

func (m configureTask) View() string {
	// applist and editor string variables are used to build out the respective views and
	// are then joined at the end.
	var applist, editor string

	if len(m.config.Applications) > 0 {
		// application list
		for k, app := range m.config.Applications {
			if app.AppName == "" {
				// app.AppName will only be empty if a new application is being created.
				// In which case, indicate that to the user.
				app.AppName = "+"
			}
			if k == m.selectedApp {
				// Show whether an item in the application list is:
				if m.state == 0 {
					// Selected
					applist += selectedFocusedItemStyle.Render(app.AppName)
				} else {
					// Being Edited
					applist += selectedItemStyle.Render(app.AppName)
				}
			} else {
				// Non of the above
				applist += itemStyle.Render(app.AppName)
			}
			applist += "\n"
		}

		// editor

		// Only show the editor and fields if viewing/editing an application.

		// display is the rendered visual of the paginator.
		var display string

		// pageStart is the index of the start of the page that m.focusedInput is on and
		// pageEnd is the end of that page.
		var pageStart, pageEnd int

		// Paginator sets display, pageStart and pageEnd
		display, pageStart, pageEnd = Pagination(len(m.inputs), inputsPerPage, m.focusedInput)

		for i := pageStart; i < pageEnd+1; i++ {
			editor += m.inputs[i].View(m.state, i == m.focusedInput)
		}

		editor = lipgloss.JoinVertical(lipgloss.Center, editorFieldsBlock.Render(editor), display)
	} else {
		// Indicate that there are no items in the application list
		applist = lipgloss.NewStyle().Foreground(darkGray).Render("No Items")
	}

	// Change the border colour of the app list/editor depending on which the user is focusing on.
	if m.state == 0 {
		applist = applistStyle.BorderForeground(lightBlue).Render(headerStyle.Render("Applications") + "\n" + applist)
		editor = editorStyle.BorderForeground(darkGray).Render(headerStyle.Render("Details") + "\n" + editor)
	} else {
		applist = applistStyle.BorderForeground(darkGray).Render(headerStyle.Render("Applications") + "\n" + applist)
		editor = editorStyle.BorderForeground(lightBlue).Render(headerStyle.Render("Details") + "\n" + editor)
	}

	return lipgloss.JoinHorizontal(lipgloss.Top,
		applist,
		editor,
	)
}

func (m configureTask) GetHelp() help.KeyMap {
	return nil
}

// nextApplication changes the selected app to the next one in the list.
// It also wraps around if the end of the list is reached.
func (m *configureTask) nextApplication() {
	if len(m.config.Applications) > 0 {
		m.setOptions(false)

		m.selectedApp = (m.selectedApp + 1) % len(m.config.Applications)

		m.setInputs(false)
	}
}

// prevApplication changes the selected app to the prev one in the list.
// It also wraps around if the start of the list is reached.
func (m *configureTask) prevApplication() {
	if len(m.config.Applications) > 0 {
		m.setOptions(false)

		m.selectedApp--
		// Wrap around
		if m.selectedApp < 0 {
			m.selectedApp = len(m.config.Applications) - 1
		}

		m.setInputs(false)
	}
}

// newApplication adds a new application to m.config.Applications and
// automatically enters the editor for the new application.
func (m *configureTask) newApplication() tea.Cmd {
	m.config.Applications = append(m.config.Applications, Options{})
	m.selectedApp = len(m.config.Applications) - 1
	m.focusedInput = 0

	m.setInputs(false)
	return m.setState(1)
}

// setState changes the state of the model.
//
// Available states:
//
//   - 0: application list
//   - 1: editor
//
// If another state is provided, the method will panic.
// The panic is for development only. It will always be caught during testing.
func (m *configureTask) setState(newState int) tea.Cmd {
	switch newState {
	case 0:
		// changing to applist
		m.setOptions(false)
		m.state = newState
		m.inputs[m.focusedInput].Blur()
		m.setInputs(false)
		return nil
	case 1:
		// changing to editor
		if len(m.config.Applications) == 0 {
			return nil
		}

		m.state = newState
		return m.inputs[m.focusedInput].Focus()
	default:
		panic(fmt.Sprintf("state: %d does not exist", newState))
	}
}

// createInputs reflects m.config.Default to create a list of inputs.
// This was done so that the Options struct can be updated in the future
// without having to manually update the UI as well.
//
// createInputs currently only supports: string, int, bool and []string.
// if the field is non of the above, then the method will panic.
// The panic is for development only. It will always be caught during testing.
//
// createInputs uses the yaml tag as the name of the field. If the yaml tag is
// set to "-" it is ignored.
//
// createInputs is only run once when the configureTask is initiated.
func (m *configureTask) createInputs() {
	t := reflect.TypeOf(m.config.Default)
	switch t.Kind() {
	case reflect.Ptr:
		t = t.Elem()
		fallthrough
	case reflect.Struct:
		numFields := t.NumField()

		m.inputs = make([]input, 0, numFields)

		for i := range numFields {
			f := t.Field(i)

			tag, ok := f.Tag.Lookup("yaml")
			if ok && tag == "-" {
				continue
			}

			tag = strings.Split(tag, ",")[0]

			// the inputs for any fields that contain required, will be set to required.
			validateTag, _ := f.Tag.Lookup("validate")

			switch f.Type.Kind() {
			case reflect.String, reflect.Int:
				m.inputs = append(m.inputs, newInput(tag, f.Name, strings.Contains(validateTag, "required"), inputTypeTextInput))
			case reflect.Bool:
				m.inputs = append(m.inputs, newInput(tag, f.Name, strings.Contains(validateTag, "required"), inputTypeBool))
			case reflect.Slice:
				m.inputs = append(m.inputs, newInput(tag, f.Name, strings.Contains(validateTag, "required"), inputTypeTextArea))
			default:
				panic(fmt.Sprintf("kind: %s is not currently supported", f.Type.Kind()))
			}
		}
	}
}

// setInputs updates the inputs with a Options struct field values.
// Passing setDefault as true will take the Default Options struct
// in the Config struct, and set the inputs to its values. Otherwise
// it will use m.selectedApp.
func (m *configureTask) setInputs(setDefault bool) {
	var v reflect.Value
	if setDefault {
		v = reflect.ValueOf(m.config.Default)
	} else {
		v = reflect.ValueOf(m.config.Applications[m.selectedApp])
	}
	for i := range m.inputs {
		f := v.FieldByName(m.inputs[i].fieldName)
		switch f.Kind() {
		case reflect.Ptr:
			f.Elem()
			fallthrough
		case reflect.Bool:
			m.inputs[i].SetValue(strconv.FormatBool(f.Bool()))
		case reflect.Int:
			m.inputs[i].SetValue(strconv.FormatInt(f.Int(), 10))
		case reflect.Slice:
			slice, ok := f.Interface().([]string)
			if !ok {
				panic("slice currently only supports []string")
			}

			if len(slice) == 0 {
				m.inputs[i].Reset()
			} else {
				m.inputs[i].SetValue(strings.Join(slice, "\n"))
			}

		case reflect.String:
			m.inputs[i].SetValue(f.String())
		}

		// Set the cursor to the end of input.
		// This is for when switching from an
		// input with a shorter value to an input
		// with a longer value to prevent the cursor
		// from being placed in the middle of the longer
		// value.
		m.inputs[i].CursorEnd()
	}
}

// setOptions updates a Options struct field values using the input values.
// Passing setDefault as true will update the Default Options struct
// in the Config struct. Otherwise it will update the selected app.
func (m *configureTask) setOptions(setDefault bool) {
	var v reflect.Value
	if setDefault {
		v = reflect.ValueOf(&m.config.Default)
	} else {
		v = reflect.ValueOf(&m.config.Applications[m.selectedApp])
	}
	v = v.Elem()
	for i := range m.inputs {
		f := v.FieldByName(m.inputs[i].fieldName)
		switch f.Kind() {
		case reflect.Ptr:
			f.Elem()
			fallthrough
		case reflect.Bool:
			if b, err := strconv.ParseBool(m.inputs[i].Value()); err == nil {
				f.SetBool(b)
			}
		case reflect.Int:
			if in, err := strconv.Atoi(m.inputs[i].Value()); err == nil {
				f.SetInt(int64(in))
			}
			// TODO: Catch int error
		case reflect.Slice:
			slice := strings.Split(m.inputs[i].Value(), "\n")

			valueSlice := reflect.MakeSlice(reflect.Indirect(reflect.ValueOf(&slice)).Type(), len(slice), len(slice))
			for i := range valueSlice.Len() {
				valueSlice.Index(i).SetString(slice[i])
			}
			f.Set(valueSlice)

		case reflect.String:
			f.SetString(m.inputs[i].Value())
		}
	}
}

// nextInput switches the focus to the next input in the editor.
// It also wraps around if the end of the list is reached.
func (m *configureTask) nextInput() tea.Cmd {
	if len(m.inputs) == 0 {
		panic("no inputs")
	}

	m.inputs[m.focusedInput].Blur()
	m.focusedInput = (m.focusedInput + 1) % len(m.inputs)
	return m.inputs[m.focusedInput].Focus()
}

// prevInput switches the focus to the prev input in the editor.
// It also wraps around if the start of the list is reached.
func (m *configureTask) prevInput() tea.Cmd {
	if len(m.inputs) == 0 {
		panic("no inputs")
	}
	m.inputs[m.focusedInput].Blur()

	m.focusedInput--
	// Wrap around
	if m.focusedInput < 0 {
		m.focusedInput = len(m.inputs) - 1
	}

	return m.inputs[m.focusedInput].Focus()
}

// input wraps the inputer interface and provides meta data for visualization and updating struct fields.
// It also provides methods for handling unique input cases.
type input struct {
	name       string // display name/yaml tag
	fieldName  string // Struct field name
	isRequired bool
	inputer
}

// Update wraps and handles the unique Update methods for all of the supported input types.
func (i *input) Update(msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd
	switch m := i.inputer.(type) {
	case *checkbox.Model:
		var b checkbox.Model
		b, cmd = m.Update(msg)
		i.inputer = &b
	case *textarea.Model:
		var in textarea.Model
		in, cmd = m.Update(msg)
		i.inputer = &in
	case *textinput.Model:
		var in textinput.Model
		in, cmd = m.Update(msg)
		i.inputer = &in
	}

	return cmd
}

func (i *input) View(state int, focused bool) string {
	var req string
	if i.isRequired {
		req = redForeground.Render("*")
	}

	if focused && state == 1 {
		// Being focused
		return fmt.Sprintf("%s%s\n%s\n%s\n",
			lightBlueForeground.Render(i.name),
			req,
			darkGrayForeground.Render(helpText[i.name]),
			i.inputer.View(),
		)
	} else if focused && state == 0 {
		// The last input that the user was editing shown while they are focused on the app list.
		return fmt.Sprintf("%s%s\n%s\n%s\n",
			darkBlueForeground.Render(i.name),
			req,
			darkGrayForeground.Render(helpText[i.name]),
			i.inputer.View(),
		)
	} else {
		// Not being focused
		return fmt.Sprintf("%s%s\n%s\n", i.name, req, i.inputer.View())
	}
}

// inputer provides a standard interface for all inputs.
type inputer interface {
	Blur()
	Focus() tea.Cmd
	Reset()
	SetValue(s string)
	Value() string
	View() string
	CursorEnd()
}

// newInput creates and returns a new input. This function is a input factory that sets the
// embedded inputer field to a concrete input struct based on provided t int type.
//
// All input customization is handled here.
//
// if argument t does not match available options, then the function will panic.
// The panic is for development only. It will always be caught during testing.
func newInput(name, fieldName string, isRequired bool, t int) input {
	i := input{
		name:       name,
		fieldName:  fieldName,
		isRequired: isRequired,
	}

	switch t {
	case inputTypeTextArea:
		ta := textarea.New()
		ta.SetHeight(3)
		i.inputer = &ta

	case inputTypeTextInput:
		ti := textinput.New()
		i.inputer = &ti

	case inputTypeBool:
		bi := checkbox.New()
		i.inputer = &bi

	default:
		panic(fmt.Sprintf("type: %d is not supported", t))
	}

	return i
}

// Pagination is a function with named return values, that renders the paginator and
// calculates the pageStartIndex and pageEndIndex using provided: totalElements, inputsPerPage and
// currentElementIndex, before returning them.
func Pagination(totalElements, inputsPerPage, currentElementIndex int) (display string, pageStartIndex int, pageEndIndex int) {
	numPages := int(math.Ceil(float64(totalElements) / float64(inputsPerPage)))

	pageStartIndex = (currentElementIndex / inputsPerPage) * inputsPerPage
	pageEndIndex = int(math.Min(float64(pageStartIndex+inputsPerPage-1), float64(totalElements-1)))

	currentPage := int(math.Ceil(float64(pageStartIndex) / float64(inputsPerPage)))

	for page := range numPages {
		if currentPage == page {
			display += lipgloss.NewStyle().Width(3).Background(lightBlue).AlignHorizontal(lipgloss.Center).Render(strconv.Itoa(page + 1))
		} else {
			display += lipgloss.NewStyle().Width(3).Foreground(darkGray).AlignHorizontal(lipgloss.Center).Render(strconv.Itoa(page + 1))
		}
	}

	return
}
