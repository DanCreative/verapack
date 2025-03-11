package verapack

import (
	"fmt"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	tm "github.com/buger/goterm"
)

type TaskStatus int

const (
	padding = 2

	Success TaskStatus = iota
	InProgress
	Failure
	Skip
	NotStarted
)

type taskState struct {
	taskStatus      TaskStatus
	spinnerPosition int
}

func (t *taskState) updateSpinnerPos() {
	if t.spinnerPosition == 3 {
		t.spinnerPosition = 0
	} else {
		t.spinnerPosition++
	}
}

type Row struct {
	ApplicationName string
	TaskList        map[string]*taskState // map where key is task name and value is taskState
	TimeTaken       string
	StartTime       time.Time
}

type header struct {
	charWidth int
	name      string
}

// ReportCard provides feedback to the user in a table format. When started, it runs in its own goroutine.
//
// Currently can't add applications to the report card while it is running.
type ReportCard struct {
	headings  []*header
	rows      []Row
	muRows    *sync.RWMutex
	doneCh    chan struct{}
	isStarted bool
}

func NewReportCard() *ReportCard {
	return &ReportCard{
		headings: []*header{
			{name: "Name", charWidth: 30},
			{name: "Package", charWidth: 10},
			{name: "Scan", charWidth: 10},
			{name: "Time Taken", charWidth: 15},
		},
		muRows: &sync.RWMutex{},
		doneCh: make(chan struct{}),
	}
}

// addApplications adds all of the applications as rows to the report table.
// It also sets Package task to skip if PackageSource is empty i.e. should not package.
//
// NB: Cannot currently add an application while the report is running.
func (r *ReportCard) addApplications(apps []Options) {
	if r.isStarted {
		panic("cannot add application to already started ReportCard")
	}

	r.muRows.Lock()
	defer r.muRows.Unlock()

	for _, app := range apps {
		taskList := map[string]*taskState{
			"Scan": {taskStatus: NotStarted},
		}

		if app.PackageSource == "" {
			taskList["Package"] = &taskState{taskStatus: Skip}
		} else {
			taskList["Package"] = &taskState{taskStatus: NotStarted}
		}

		r.rows = append(r.rows, Row{
			ApplicationName: app.AppName,
			TaskList:        taskList,
		})
	}
}

// Start runs the ReportCard as a goroutine and returns the done channel that should be used to signal done.
func (r *ReportCard) Start() {
	go func() {
		for {
			r.Print()

			time.Sleep(250 * time.Millisecond)

			select {
			case <-r.doneCh:
				r.Print()
				close(r.doneCh)
				return
			default:
			}
		}
	}()
}

// Update allows other goroutines to update the progress of applications on the ReportCard.
//
// The first call to Update will set the StartTime for the application row. (The first call will
// always be to set it to InProgress)
//
// On the last call, caller should set isDone to true.
func (r *ReportCard) Update(rowIndex int, taskName string, newStatus TaskStatus, isDone bool) {
	r.muRows.Lock()
	defer r.muRows.Unlock()

	if r.rows[rowIndex].StartTime.IsZero() {
		r.rows[rowIndex].StartTime = time.Now()
	}

	if isDone {
		r.rows[rowIndex].TimeTaken = fmt.Sprintf("%.2fs", time.Since(r.rows[rowIndex].StartTime).Seconds())
	}

	r.rows[rowIndex].TaskList[taskName] = &taskState{
		taskStatus: newStatus,
	}
}

// Stop stops the ReportCard goroutine. It does this by sending a value on the done channel.
// It also blocks and waits for the last print to finish.
func (r *ReportCard) Stop() {
	r.doneCh <- struct{}{}
	<-r.doneCh
}

// Print prints the table using goterm.
func (r *ReportCard) Print() {
	tm.Clear()
	tm.MoveCursor(1, 1)
	rowLine := generateRowLine(r.headings)
	tm.Println(rowLine)
	tm.Println(tm.Bold(generateHeaderRow(r.headings)))
	tm.Println(rowLine)
	for _, row := range r.rows {
		tm.Println(generateRow(r.headings, row))
	}
	tm.Println(rowLine)

	tm.Flush()

}

// generateRow is a helper function that returns a line as a string.
// It also updates the spinner/loading screen" positions/frames.
func generateRow(headings []*header, row Row) string {
	r := "|"

	for i, header := range headings {
		if i == 0 {
			// Handle Application Name column
			r += generateContent(row.ApplicationName, header.charWidth)
			continue
		}

		if i == len(headings)-1 {
			// Handle Time Taken column
			r += generateContent(row.TimeTaken, header.charWidth)
			continue
		}

		if state := row.TaskList[header.name]; state.taskStatus == InProgress {
			state.updateSpinnerPos()
			r += generateContent(parseSpinner(state.spinnerPosition), header.charWidth)
		} else {
			r += generateContent(parseTaskStatus(state.taskStatus), header.charWidth)
		}
	}

	return r
}

// parseSpinner takes the current position/frame of a spinner and returns
// the string representation.
func parseSpinner(id int) string {
	switch id {
	case 0:
		return "|"
	case 1:
		return "/"
	case 2:
		return "-"
	case 3:
		return "\\"
	}

	return ""
}

// parseTaskStatus takes a taskStatus and returns the string representation.
func parseTaskStatus(taskStatus TaskStatus) string {
	switch taskStatus {
	case Success:
		return "OK"
	case Failure:
		return "NOK"
	case Skip:
		return "-"
	case NotStarted:
		return "TODO"
	case InProgress:
		return "WIP"
	}
	return ""
}

// generateHeaderRow is a helper function that returns a header row as a string.
func generateHeaderRow(headings []*header) string {
	r := "|"
	// var r string

	for _, header := range headings {
		r += generateContent(header.name, header.charWidth)
	}

	return r
}

// generateRowLine is a helper function that returns a table vertical line as a string.
func generateRowLine(headings []*header) string {
	var l int
	var rowLine string

	for _, header := range headings {
		l += header.charWidth + padding*2
	}

	l += len(headings) + 1

	rowLine = "|" + strings.Repeat("-", l-2) + "|"
	return rowLine
}

// generateContent is a helper-helper function that takes a value for the column and the column width,
// and returns the formatted column (including lines and padding).
//
// If the length of the value exceeds the column width, it is trimmed and "..." is added to the end to
// show overflow.
func generateContent(colValue string, colWidth int) string {
	valueLen := utf8.RuneCount([]byte(colValue))
	if valueLen > colWidth {
		colValue = colValue[:colWidth-3]
		colValue += "..."
		valueLen = colWidth
	}

	r := strings.Repeat(" ", padding) + colValue + strings.Repeat(" ", colWidth-valueLen+padding) + "|"

	return r
}
