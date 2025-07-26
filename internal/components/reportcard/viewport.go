package reportcard

import (
	"github.com/DanCreative/verapack/internal/components/viewport"
	tea "github.com/charmbracelet/bubbletea"
)

// Viewport is used by the [Model] to render the output window.
type Viewport interface {
	// Init initializes the [Viewport]. It sets the height, width and inputData.
	//
	// Init can also be used to update the inputData.
	Init(width int, height int, inputData any) tea.Cmd
	Update(msg tea.Msg) (Viewport, tea.Cmd)
	View() string

	// HasBeenInitialized returns whether the Viewport has been initialized.
	HasBeenInitialized() bool

	// SetContent inputs data that the implementor should parse.
	SetContent(inputData any)

	// SetDimensions sets the width and height of the viewport.
	SetDimensions(width int, height int)
	// ShouldShowScrollBar returns a bool that indicates whether the reportcard
	// should show the scrollbar.
	ShouldShowScrollBar() bool
	// AtTop returns a bool that indicates whether the viewport is at the top.
	AtTop() bool
	// AtBottom returns a bool that indicates whether the viewport is at the bottom.
	AtBottom() bool
	// LineDown moves the view down by the given number of lines.
	LineDown(n int) tea.Cmd
	// LineUp moves the view up by the given number of lines.
	LineUp(n int) tea.Cmd
	// HalfViewUp moves the view up by half the height of the viewport.
	HalfViewUp() tea.Cmd
	// HalfViewDown moves the view down by half the height of the viewport.
	HalfViewDown() tea.Cmd
	// ViewUp moves the view up by one height of the viewport. Basically, "page up".
	ViewUp() tea.Cmd
	// ViewUp moves the view down by one height of the viewport. Basically, "page down".
	ViewDown() tea.Cmd
}

var _ Viewport = (*DefaultViewport)(nil)

type DefaultViewport struct {
	hasBeenInitialized  bool
	shouldShowScrollBar bool
	viewport            viewport.Model
}

func (d *DefaultViewport) Init(width int, height int, inputData any) tea.Cmd {
	if !d.hasBeenInitialized {
		d.viewport = viewport.New(width, height)
		d.viewport.YPosition = 0
		d.hasBeenInitialized = true

		d.SetDimensions(width, height)
		d.SetContent(inputData)
	}

	return nil
}

func (d *DefaultViewport) SetContent(inputData any) {
	if val, ok := inputData.(string); ok {
		d.viewport.SetContent(val)
		d.viewport.YOffset = 0
		d.viewport.SetWrappedLines(d.viewport.Width)
	}

	d.shouldShowScrollBar = d.viewport.VisibleLineCount() < d.viewport.TotalLineCount()
}

func (d *DefaultViewport) SetDimensions(width int, height int) {
	d.viewport.Height, d.viewport.Width = height, width
}

// Update handles the mouse events and initialization on the [viewport.Model].
func (d DefaultViewport) Update(msg tea.Msg) (Viewport, tea.Cmd) {
	var cmd tea.Cmd
	d.viewport, cmd = d.viewport.Update(msg)
	return &d, cmd
}

func (d DefaultViewport) HasBeenInitialized() bool {
	return d.hasBeenInitialized
}

func (d DefaultViewport) View() string {
	if !d.hasBeenInitialized {
		// Should never show, output.View is only possible to run
		// once an error occurs. That will usually only happen after
		// a couple of mins.
		return "\n  Initializing..."
	}

	return d.viewport.View()
}

// ShouldShowScrollBar returns a bool that indicates whether the reportcard
// should show the scrollbar.
func (d DefaultViewport) ShouldShowScrollBar() bool {
	return d.shouldShowScrollBar
}

// AtTop returns a bool that indicates whether the viewport is at the top.
func (d DefaultViewport) AtTop() bool {
	return d.viewport.AtTop()
}

// AtBottom returns a bool that indicates whether the viewport is at the bottom.
func (d DefaultViewport) AtBottom() bool {
	// Current implementation:
	// m.output.viewport.YOffset+1 > m.output.viewport.TotalLineCount()-m.output.viewport.VisibleLineCount()
	return d.viewport.AtBottom()
}

// LineDown moves the view down by the given number of lines.
func (d *DefaultViewport) LineDown(n int) tea.Cmd {
	var cmd tea.Cmd
	lines := d.viewport.LineDown(1)

	if d.viewport.HighPerformanceRendering {
		cmd = viewport.ViewDown(d.viewport, lines)
	}

	return cmd
}

// LineUp moves the view up by the given number of lines.
func (d *DefaultViewport) LineUp(n int) tea.Cmd {
	var cmd tea.Cmd
	lines := d.viewport.LineUp(1)

	if d.viewport.HighPerformanceRendering {
		cmd = viewport.ViewUp(d.viewport, lines)
	}

	return cmd
}

// HalfViewUp moves the view up by half the height of the viewport.
func (d *DefaultViewport) HalfViewUp() tea.Cmd {
	var cmd tea.Cmd
	lines := d.viewport.HalfViewUp()

	if d.viewport.HighPerformanceRendering {
		cmd = viewport.ViewUp(d.viewport, lines)
	}

	return cmd
}

// HalfViewDown moves the view down by half the height of the viewport.
func (d *DefaultViewport) HalfViewDown() tea.Cmd {
	var cmd tea.Cmd
	lines := d.viewport.HalfViewDown()

	if d.viewport.HighPerformanceRendering {
		cmd = viewport.ViewDown(d.viewport, lines)
	}

	return cmd
}

// ViewUp moves the view up by one height of the viewport. Basically, "page up".
func (d *DefaultViewport) ViewUp() tea.Cmd {
	var cmd tea.Cmd
	lines := d.viewport.ViewUp()

	if d.viewport.HighPerformanceRendering {
		cmd = viewport.ViewUp(d.viewport, lines)
	}

	return cmd
}

// ViewUp moves the view down by one height of the viewport. Basically, "page down".
func (d *DefaultViewport) ViewDown() tea.Cmd {
	var cmd tea.Cmd
	lines := d.viewport.ViewDown()

	if d.viewport.HighPerformanceRendering {
		cmd = viewport.ViewDown(d.viewport, lines)
	}

	return cmd
}
