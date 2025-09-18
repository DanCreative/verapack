package reportcard

import (
	"fmt"
	"reflect"
	"testing"
)

type update func(r *Row, msg TaskResultMsg)

func uf(r *Row, msg TaskResultMsg) {
	r.update(msg)
}

type arg struct {
	uf  update
	msg TaskResultMsg
}

func Test_row_update(t *testing.T) {
	tests := []struct {
		name          string
		r             *Row
		args          []arg
		wantRowStatus RowStatus
		wantRowTasks  []Task
	}{
		{
			name: "on task one, success",
			r: &Row{
				status:           RowNotStarted,
				columnsReference: map[string]int{"t1": 0, "t2": 1, "t3": 2},
				tasks:            []Task{{status: NotStarted, name: "t1"}, {status: NotStarted, name: "t2"}, {status: NotStarted, name: "t3"}},
			},
			args:          []arg{{uf: uf, msg: TaskResultMsg{Status: Success}}},
			wantRowStatus: RowStarted,
			wantRowTasks:  []Task{{name: "t1", status: Success}, {name: "t2", status: InProgress}, {name: "t3", status: NotStarted}},
		},
		{
			name: "all tasks skipped, success",
			r: &Row{
				status:           RowNotStarted,
				columnsReference: map[string]int{"t1": 0, "t2": 1, "t3": 2},
				tasks:            []Task{{status: Skip, name: "t1"}, {status: Skip, name: "t2"}, {status: Skip, name: "t3"}},
			},
			args:          []arg{{uf: uf, msg: TaskResultMsg{Status: Success}}},
			wantRowStatus: RowSuccess,
			wantRowTasks:  []Task{{name: "t1", status: Skip}, {name: "t2", status: Skip}, {name: "t3", status: Skip}},
		},
		{
			name: "skipped 2 starting tasks, success",
			r: &Row{
				status:           RowNotStarted,
				columnsReference: map[string]int{"t1": 0, "t2": 1, "t3": 2},
				tasks:            []Task{{status: Skip, name: "t1"}, {status: Skip, name: "t2"}, {status: NotStarted, name: "t3"}},
			},
			args:          []arg{{uf: uf, msg: TaskResultMsg{Status: Success}}},
			wantRowStatus: RowSuccess,
			wantRowTasks:  []Task{{name: "t1", status: Skip}, {name: "t2", status: Skip}, {name: "t3", status: Success}},
		},
		{
			name: "skipped tasks in between, success",
			r: &Row{
				status:           RowNotStarted,
				columnsReference: map[string]int{"t1": 0, "t2": 1, "t3": 2, "t4": 3},
				tasks:            []Task{{status: NotStarted, name: "t1"}, {status: Skip, name: "t2"}, {status: Skip, name: "t3"}, {status: NotStarted, name: "t4"}},
			},
			wantRowStatus: RowStarted,
			wantRowTasks:  []Task{{status: Success, name: "t1"}, {status: Skip, name: "t2"}, {status: Skip, name: "t3"}, {status: InProgress, name: "t4"}},
			args:          []arg{{uf: uf, msg: TaskResultMsg{Status: Success}}},
		},
		{
			name: "failure occurred",
			r: &Row{
				status:           RowNotStarted,
				columnsReference: map[string]int{"t1": 0, "t2": 1, "t3": 2, "t4": 3},
				tasks:            []Task{{status: NotStarted, name: "t1"}, {status: Skip, name: "t2"}, {status: NotStarted, name: "t3"}, {status: NotStarted, name: "t4"}},
			},
			wantRowStatus: RowFailure,
			wantRowTasks:  []Task{{status: Success, name: "t1"}, {status: Skip, name: "t2"}, {status: Failure, name: "t3"}, {status: Skip, name: "t4"}},
			args:          []arg{{uf: uf, msg: TaskResultMsg{Status: Success}}, {uf: uf, msg: TaskResultMsg{Status: Failure}}},
		},
		{
			name: "warning occurred",
			r: &Row{
				status:           RowNotStarted,
				columnsReference: map[string]int{"t1": 0, "t2": 1, "t3": 2, "t4": 3},
				tasks:            []Task{{status: NotStarted, name: "t1"}, {status: Skip, name: "t2"}, {status: NotStarted, name: "t3"}, {status: NotStarted, name: "t4"}},
			},
			wantRowStatus: RowWarning,
			wantRowTasks:  []Task{{status: Warning, name: "t1"}, {status: Skip, name: "t2"}, {status: Success, name: "t3"}, {status: Success, name: "t4"}},
			args:          []arg{{uf: uf, msg: TaskResultMsg{Status: Warning}}, {uf: uf, msg: TaskResultMsg{Status: Success}}, {uf: uf, msg: TaskResultMsg{Status: Success}}},
		},
		{
			name: "can run anyway, still in progress",
			r: &Row{
				status:           RowNotStarted,
				columnsReference: map[string]int{"t1": 0, "t2": 1, "t3": 2, "t4": 3},
				tasks:            []Task{{status: NotStarted, name: "t1"}, {status: NotStarted, name: "t2"}, {status: NotStarted, name: "t3", shouldRunAnywayFor: []string{"t1", "t2"}}, {status: NotStarted, name: "t4"}},
			},
			args:          []arg{{uf: uf, msg: TaskResultMsg{Status: Failure}}},
			wantRowStatus: RowStarted,
			wantRowTasks:  []Task{{status: Failure, name: "t1"}, {status: Skip, name: "t2"}, {status: InProgress, name: "t3", shouldRunAnywayFor: []string{"t1", "t2"}}, {status: Skip, name: "t4"}},
		},
		{
			name: "can run anyway, after remaining tasks complete",
			r: &Row{
				status:           RowNotStarted,
				columnsReference: map[string]int{"t1": 0, "t2": 1, "t3": 2, "t4": 3},
				tasks:            []Task{{status: NotStarted, name: "t1"}, {status: NotStarted, name: "t2"}, {status: NotStarted, name: "t3", shouldRunAnywayFor: []string{"t1", "t2"}}, {status: NotStarted, name: "t4"}},
			},
			args:          []arg{{uf: uf, msg: TaskResultMsg{Status: Failure}}, {uf: uf, msg: TaskResultMsg{Status: Success}}},
			wantRowStatus: RowFailure,
			wantRowTasks:  []Task{{status: Failure, name: "t1"}, {status: Skip, name: "t2"}, {status: Success, name: "t3", shouldRunAnywayFor: []string{"t1", "t2"}}, {status: Skip, name: "t4"}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.r.start()

			for _, cmd := range tt.args {
				cmd.uf(tt.r, cmd.msg)
			}

			if !reflect.DeepEqual(tt.r.status, tt.wantRowStatus) {
				t.Errorf("row.update() = %v, want %v", tt.r.status, tt.wantRowStatus)
			}

			if !reflect.DeepEqual(tt.r.tasks, tt.wantRowTasks) {
				t.Errorf("row.update(), row.Tasks = %+v, \nwant %+v", tt.r.tasks, tt.wantRowTasks)
			}
			fmt.Printf("row: %+v\n", tt.r)
		})
	}
}
