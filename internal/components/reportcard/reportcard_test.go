package reportcard

import "testing"

func newTestSelector[T any](selectables [][]*T, selectedCol, selectedRow int) *selector[T] {
	return &selector[T]{
		selectedItemRow:    selectedRow,
		selectedItemColumn: selectedCol,
		selectableItems:    selectables,
	}
}

func Test_selector_MoveCursor(t *testing.T) {
	type args struct {
		direction cursorDirection
	}
	tests := []struct {
		name    string
		s       *selector[struct{}]
		args    args
		wantRow int
		wantCol int
	}{
		{
			name: "Move 1 left",
			s:    newTestSelector([][]*struct{}{{nil, &struct{}{}, nil}}, 2, 0),
			args: args{
				direction: left,
			},
			wantRow: 0,
			wantCol: 1,
		},
		{
			name: "Jump column(s) left",
			s:    newTestSelector([][]*struct{}{{&struct{}{}, nil, &struct{}{}}}, 2, 0),
			args: args{
				direction: left,
			},
			wantRow: 0,
			wantCol: 0,
		},
		{
			name: "No move left",
			s:    newTestSelector([][]*struct{}{{nil, nil, &struct{}{}}}, 2, 0),
			args: args{
				direction: left,
			},
			wantRow: 0,
			wantCol: 2,
		},
		{
			name: "No move left border",
			s:    newTestSelector([][]*struct{}{{&struct{}{}}}, 0, 0),
			args: args{
				direction: left,
			},
			wantRow: 0,
			wantCol: 0,
		},
		{
			name: "Move 1 right",
			s:    newTestSelector([][]*struct{}{{&struct{}{}, &struct{}{}, nil}}, 0, 0),
			args: args{
				direction: right,
			},
			wantRow: 0,
			wantCol: 1,
		},
		{
			name: "Jump column(s) right",
			s:    newTestSelector([][]*struct{}{{&struct{}{}, nil, &struct{}{}}}, 0, 0),
			args: args{
				direction: right,
			},
			wantRow: 0,
			wantCol: 2,
		},
		{
			name: "No move right",
			s:    newTestSelector([][]*struct{}{{&struct{}{}, nil, nil}}, 0, 0),
			args: args{
				direction: right,
			},
			wantRow: 0,
			wantCol: 0,
		},
		{
			name: "No move right border",
			s:    newTestSelector([][]*struct{}{{nil, nil, &struct{}{}}}, 2, 0),
			args: args{
				direction: right,
			},
			wantRow: 0,
			wantCol: 2,
		},
		{
			name: "Move 1 row down same column",
			s:    newTestSelector([][]*struct{}{{nil, &struct{}{}, nil}, {nil, &struct{}{}, nil}}, 1, 0),
			args: args{
				direction: down,
			},
			wantRow: 1,
			wantCol: 1,
		},
		{
			name: "Move 1 row down different column",
			s:    newTestSelector([][]*struct{}{{nil, &struct{}{}, nil}, {nil, nil, &struct{}{}}}, 1, 0),
			args: args{
				direction: down,
			},
			wantRow: 1,
			wantCol: 2,
		},
		{
			name: "Move 1 row down with multiple rows and different column",
			s:    newTestSelector([][]*struct{}{{nil, &struct{}{}, nil}, {&struct{}{}, nil, nil}, {nil, nil, &struct{}{}}}, 1, 0),
			args: args{
				direction: down,
			},
			wantRow: 1,
			wantCol: 0,
		},
		{
			name: "Jump rows down with different column",
			s:    newTestSelector([][]*struct{}{{nil, &struct{}{}, nil}, {nil, nil, nil}, {nil, nil, &struct{}{}}}, 1, 0),
			args: args{
				direction: down,
			},
			wantRow: 2,
			wantCol: 2,
		},
		{
			name: "No move down",
			s:    newTestSelector([][]*struct{}{{nil, nil, nil}, {nil, &struct{}{}, nil}, {nil, nil, nil}}, 1, 1),
			args: args{
				direction: down,
			},
			wantRow: 1,
			wantCol: 1,
		},
		{
			name: "No move down border",
			s:    newTestSelector([][]*struct{}{{nil, &struct{}{}, nil}}, 1, 0),
			args: args{
				direction: down,
			},
			wantRow: 0,
			wantCol: 1,
		},
		{
			name: "Move 1 row up same column",
			s:    newTestSelector([][]*struct{}{{nil, &struct{}{}, nil}, {nil, &struct{}{}, nil}}, 1, 1),
			args: args{
				direction: up,
			},
			wantRow: 0,
			wantCol: 1,
		},
		{
			name: "Move 1 row up different column",
			s:    newTestSelector([][]*struct{}{{nil, &struct{}{}, nil}, {nil, nil, &struct{}{}}}, 2, 1),
			args: args{
				direction: up,
			},
			wantRow: 0,
			wantCol: 1,
		},
		{
			name: "Jump rows up with different column",
			s:    newTestSelector([][]*struct{}{{nil, &struct{}{}, nil}, {nil, nil, nil}, {nil, nil, &struct{}{}}}, 2, 2),
			args: args{
				direction: up,
			},
			wantRow: 0,
			wantCol: 1,
		},
		{
			name: "No move up",
			s:    newTestSelector([][]*struct{}{{nil, nil, nil}, {nil, &struct{}{}, nil}, {nil, nil, nil}}, 1, 1),
			args: args{
				direction: up,
			},
			wantRow: 1,
			wantCol: 1,
		},
		{
			name: "No move up border",
			s:    newTestSelector([][]*struct{}{{nil, &struct{}{}, nil}}, 1, 0),
			args: args{
				direction: up,
			},
			wantRow: 0,
			wantCol: 1,
		},
		{
			name: "Move 1 up down with multiple rows and different column",
			s:    newTestSelector([][]*struct{}{{nil, &struct{}{}, nil}, {nil, nil, nil}, {nil, nil, &struct{}{}}}, 2, 2),
			args: args{
				direction: up,
			},
			wantRow: 0,
			wantCol: 1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, gotRow, gotCol, _ := tt.s.MoveCursor(tt.args.direction)
			if gotRow != tt.wantRow {
				t.Errorf("selector.MoveCursor() gotRow = %v, want %v", gotRow, tt.wantRow)
			}
			if gotCol != tt.wantCol {
				t.Errorf("selector.MoveCursor() gotCol = %v, want %v", gotCol, tt.wantCol)
			}
		})
	}
}
