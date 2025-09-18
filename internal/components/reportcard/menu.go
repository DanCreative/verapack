package reportcard

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
)

// CustomAction allows the caller to implement a custom action for a specific [row], that will be ingested by tea as a [tea.Cmd].
type CustomAction func(rowIndex int, row Row) tea.Cmd

// KeyPath is the full path of node names to a specific node in the [menuTree].
type KeyPath []string

// menuTree is a tree of [MenuNode]s that can be traversed with specified key pressed.
//
// It provided functions to traverse the tree, match keys and handle [CustomAction]s.
type menuTree struct {
	children       map[string]*MenuNode // Highest depth of children.
	cursorLocation *MenuNode            // Current node that the tree is pointing to. Will be nil if at the top depth.
	matchLocation  *MenuNode            // Set when a child's key is matched.
	backButton     key.Binding          // A button to go up a level in the tree.
}

type EnableKeyArg struct {
	Path   KeyPath // Path to the node.
	Enable bool    // Should enable or disable the associated key.
}

// MenuNode is a node in the menuTree.
//
// Currently it can be either of type folder or type action.
// Folders will contain children and Actions will contain a [CustomAction]
type MenuNode struct {
	name     string
	path     string
	nodeType int // 0 folder, 1 action
	parent   *MenuNode
	children map[string]*MenuNode
	key      key.Binding
	action   CustomAction
	index    int
}

// ToTop moves the [menuTree]'s cursor to the top depth.
func (m *menuTree) ToTop() {
	m.cursorLocation = nil
}

// EnableKeys enables or disables the keys for all of the matched nodes in the tree.
func (m *menuTree) EnableKeys(args ...EnableKeyArg) {
argsLoop:
	for _, arg := range args {
		if len(arg.Path) < 1 {
			continue
		}

		var n *MenuNode

		if v, ok := m.children[arg.Path[0]]; ok {
			n = v

		} else {
			continue
		}

		for k, step := range arg.Path[1:] {
			if v, ok := n.children[step]; ok {
				n = v
				if k+1 == len(arg.Path[1:]) {
					n.key.SetEnabled(arg.Enable)

					if !arg.Enable && m.cursorLocation == n {
						// If the key for the current folder is disabled, go up a level.
						m.Ascend()
					}
				}

			} else {
				continue argsLoop
			}
		}
	}
}

// GetNode returns a [MenuNode] that matches the given path.
func (m *menuTree) GetNode(path KeyPath) *MenuNode {
	if len(path) < 1 {
		return nil
	}

	var n *MenuNode

	if v, ok := m.children[path[0]]; ok {
		n = v
	} else {
		return nil
	}

	for _, step := range path[1:] {
		if v, ok := n.children[step]; ok {
			n = v
		} else {
			return nil
		}
	}

	return n
}

// FullHelp implements the [help.KeyMap] interface.
func (m *menuTree) FullHelp() [][]key.Binding {
	if m.cursorLocation != nil {
		keys := m.cursorLocation.ShortHelp()
		keys[0] = m.backButton

		return [][]key.Binding{keys}
	} else {
		return [][]key.Binding{m.shortHelp()}
	}
}

// ShortHelp implements the [help.KeyMap] interface.
func (m *menuTree) ShortHelp() []key.Binding {
	if m.cursorLocation != nil {
		keys := m.cursorLocation.ShortHelp()
		keys[0] = m.backButton

		return keys
	} else {
		return m.shortHelp()
	}
}

// shortHelp is used when the tree is at the top level.
func (m *menuTree) shortHelp() []key.Binding {
	keys := make([]key.Binding, len(m.children))

	for _, child := range m.children {
		keys[child.index] = child.key
	}

	return keys
}

// MatchBack tests if the keystroke matches the [menuTree].backButton.
func (m *menuTree) MatchBack(msg tea.KeyMsg) bool {
	return key.Matches(msg, m.backButton)
}

// MatchChild tests if the keystroke matches a child at the current level.
// If a child is matched, it will be set to [menuTree].matchLocation.
func (m *menuTree) MatchChild(msg tea.KeyMsg) bool {
	var children map[string]*MenuNode

	if m.cursorLocation == nil {
		children = m.children
	} else {
		children = m.cursorLocation.children
	}

	for _, child := range children {
		if key.Matches(msg, child.key) {
			m.matchLocation = child
			return true
		}
	}

	return false
}

// GetMatch returns the currently matched [MenuNode].
func (m *menuTree) GetMatch() *MenuNode {
	return m.matchLocation
}

// DescendToMatchedFolder sets the matched child as the cursor location if it
// is a folder.
//
// GetMatch().GetType() can be used to confirm that the match is a folder.
func (m *menuTree) DescendToMatchedFolder() {
	if m.matchLocation != nil && m.matchLocation.nodeType == 0 {
		m.cursorLocation = m.matchLocation
		m.matchLocation = nil
	}
}

// Ascend sets the cursor location to the parent of the current folder [MenuNode].
// That will either be a parent [MenuNode] or the [menuTree] if on level 1.
func (m *menuTree) Ascend() {
	if m.cursorLocation == nil {
		// Already at the top
		return
	}

	if m.cursorLocation.parent != nil {
		// Not at the top
		m.cursorLocation = m.cursorLocation.parent
		return
	}

	// On level 1, go to top
	m.ToTop()
}

// ShortHelp implements the [help.KeyMap] interface.
// Called by [menuTree].ShortHelp() and [menuTree].FullHelp().
func (m *MenuNode) ShortHelp() []key.Binding {
	keys := make([]key.Binding, len(m.children)+1)

	for _, child := range m.children {
		keys[child.index+1] = child.key
	}

	return keys
}

// GetType returns the type of the MenuNode.
// Can be:
//   - Folder (0)
//   - Action (1)
func (m *MenuNode) GetType() int {
	return m.nodeType
}

// GetAction returns the CustomAction of the MenuNode.
// Will be nil if the MenuNode is a folder.
func (m *MenuNode) GetAction() CustomAction {
	return m.action
}

// NewMenuTree creates a new menuTree with the provided children and default back button.
func NewMenuTree(children map[string]*MenuNode, backButton key.Binding) *menuTree {
	m := &menuTree{
		children:   children,
		backButton: backButton,
	}

	for name, child := range m.children {
		child.name = name

		if child.index >= len(m.children) {
			panic(fmt.Sprintf("children[n].index cannot >= the length of children. children[%s].index is %d", child.name, child.index))
		}
	}

	setChildPaths("", m.children)

	return m
}

// NewFolderMenuNode creates a new MenuNode.
//
// key will be used to match against the user's keystrokes.
// index will be the node's order/index when being displayed. It must be unique at the current level,
// and it can't match or exceed the total number of children at the current level.
func NewFolderMenuNode(children map[string]*MenuNode, index int, key key.Binding) *MenuNode {
	m := &MenuNode{
		nodeType: 0,
		parent:   nil,
		key:      key,
		index:    index,
		action:   nil,
		children: children,
	}

	duplicates := map[int]bool{}

	for name, child := range m.children {
		child.parent = m
		child.name = name

		if child.index >= len(m.children) {
			panic(fmt.Sprintf("children[n].index cannot >= the length of children. children[%s].index == %d, for node '%s'", child.name, child.index, m.name))
		}

		if duplicates[child.index] {
			panic(fmt.Sprintf("children[n].index must be unique, for node '%s'", m.name))
		} else {
			duplicates[child.index] = true
		}
	}

	return m
}

// NewActionMenuNode creates a new MenuNode.
//
// index will be the node's order/index when being displayed. It must be unique at the current level,
// and it can't match or exceed the total number of children at the current level.
func NewActionMenuNode(key key.Binding, index int, action CustomAction) *MenuNode {
	return &MenuNode{
		nodeType: 1,
		key:      key,
		index:    index,
		parent:   nil,
		action:   nil,
		children: nil,
	}

}

func (k KeyPath) String() string {
	return strings.Join(k, ">")
}

// setChildPaths recursively sets the absolute paths for the provided children.
func setChildPaths(path string, children map[string]*MenuNode) {
	for _, child := range children {
		if path == "" {
			child.path = child.name
		} else {
			child.path = path + "." + child.name
		}

		if child.nodeType == 1 {
			continue
		}

		setChildPaths(child.path, child.children)
	}
}
