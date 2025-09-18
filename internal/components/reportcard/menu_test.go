package reportcard

import (
	"testing"

	"github.com/charmbracelet/bubbles/key"
)

func Test_menuTree_GetNode(t *testing.T) {
	a1 := NewFolderMenuNode(nil, 0, key.NewBinding(key.WithKeys("k"), key.WithDisabled()))
	c2 := NewActionMenuNode(key.Binding{}, 1, nil)
	a2 := NewFolderMenuNode(map[string]*MenuNode{
		"b1": NewFolderMenuNode(map[string]*MenuNode{
			"c1": NewActionMenuNode(key.Binding{}, 0, nil),
			"c2": c2,
			"c3": NewActionMenuNode(key.Binding{}, 2, nil),
		}, 0, key.Binding{}),
		"b2": NewActionMenuNode(key.Binding{}, 1, nil),
	}, 0, key.Binding{})

	tests := []struct {
		name string // description of this test case
		// Named input parameters for receiver constructor.
		children map[string]*MenuNode
		// Named input parameters for target function.
		path []string
		want *MenuNode
	}{
		{
			name: "get depth 1",
			children: map[string]*MenuNode{
				"a1": a1,
			},
			path: []string{"a1"},
			want: a1,
		},
		{
			name: "get nth depth",
			children: map[string]*MenuNode{
				"a2": a2,
			},
			path: []string{"a2", "b1", "c2"},
			want: c2,
		},
		{
			name: "empty path",
			children: map[string]*MenuNode{
				"a2": a2,
			},
			path: []string{},
			want: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := NewMenuTree(tt.children, key.Binding{})
			got := m.GetNode(tt.path)

			if got == nil {
				if got != tt.want {
					t.Errorf("GetNode() = nil, want %+v (%p)", tt.want, tt.want)
				}

				return
			}

			if got.name != tt.want.name {
				t.Errorf("GetNode() = %+v (%p), want %+v (%p)", got, got, tt.want, tt.want)
			}
		})
	}
}
