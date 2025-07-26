package reportcard

type direction int

const (
	up direction = iota
	right
	down
	left
)

// selector stores all of the results and handles logic for moving between selected results.
type selector[T any] struct {
	hasChanged         bool
	notFirst           bool
	selectedItemRow    int
	selectedItemColumn int
	selectableItems    [][]*T // using pointer to easily check if selected
}

// MoveCursor moves the selector cursor in the direction provided for step int provided.
//
// It returns the currently selected item if it did not move and the new item if it did.
// It also returns the row and column index, and a bool indicating whether it moved or not.
func (s *selector[T]) MoveCursor(direction direction, step int) (T, int, int, bool) {
	var didMove bool

	switch direction {
	case up:
	upOuter:
		for i := s.selectedItemRow - step; i >= 0; i-- {
			if s.selectableItems[i][s.selectedItemColumn] != nil {
				s.selectedItemRow = i
				didMove = true
				break upOuter
			} else {
				for j := range len(s.selectableItems[i]) {
					if s.selectableItems[i][j] != nil {
						s.selectedItemColumn = j
						s.selectedItemRow = i
						didMove = true
						break upOuter
					}
				}
			}
		}
	case down:
	downOuter:
		for i := s.selectedItemRow + step; i < len(s.selectableItems); i++ {
			if s.selectableItems[i][s.selectedItemColumn] != nil {
				s.selectedItemRow = i
				didMove = true
				break downOuter

			} else {
				for j := range len(s.selectableItems[i]) {
					if s.selectableItems[i][j] != nil {
						s.selectedItemColumn = j
						s.selectedItemRow = i
						didMove = true
						break downOuter
					}
				}
			}
		}
	case right:
		for i := s.selectedItemColumn + step; i < len(s.selectableItems[s.selectedItemRow]); i++ {
			if s.selectableItems[s.selectedItemRow][i] != nil {
				s.selectedItemColumn = i
				didMove = true
				break
			}
		}
	case left:
		for i := s.selectedItemColumn - step; i >= 0; i-- {
			if s.selectableItems[s.selectedItemRow][i] != nil {
				s.selectedItemColumn = i
				didMove = true
				break
			}
		}
	}

	if didMove {
		s.hasChanged = true
	}

	return *s.selectableItems[s.selectedItemRow][s.selectedItemColumn], s.selectedItemRow, s.selectedItemColumn, didMove
}

func (s *selector[T]) AddSelectable(item T, row, col int) {
	s.selectableItems[row][col] = &item

	if !s.notFirst {
		s.selectedItemRow, s.selectedItemColumn = row, col
		s.notFirst = true
	}
}

func (s *selector[T]) GetSelected() (item *T, row, col int) {
	item, row, col = s.selectableItems[s.selectedItemRow][s.selectedItemColumn], s.selectedItemRow, s.selectedItemColumn
	return
}

func (s *selector[T]) SetSelected(row, col int) *T {
	if s.selectedItemColumn != col || s.selectedItemRow != row {
		s.hasChanged = true
	}

	s.selectedItemRow = row
	s.selectedItemColumn = col

	return s.selectableItems[s.selectedItemRow][s.selectedItemColumn]
}

// SetSelectedInRange finds and sets the first selectable item within the range.
//
// startIndex is inclusive and endIndex is exclusive.
func (s *selector[T]) SetSelectedInRange(startIndex, endIndex int) (item *T, row, col int, found bool) {
	r, c, found := s.GetSelectedInRange(startIndex, endIndex)

	if found {
		if s.selectedItemColumn != c || s.selectedItemRow != r {
			s.hasChanged = true
		}

		s.selectedItemColumn = c
		s.selectedItemRow = r
	}

	item, row, col = s.selectableItems[s.selectedItemRow][s.selectedItemColumn], s.selectedItemRow, s.selectedItemColumn
	return
}

func (s *selector[T]) GetSelectedInRange(startIndex, endIndex int) (row, col int, found bool) {
	for i := startIndex; i < endIndex; i++ {
		for j := 0; j < len(s.selectableItems[i]); j++ {
			if s.selectableItems[i][j] != nil {
				col = j
				row = i
				found = true
				return
			}
		}
	}
	return
}

// HasChanged returns whether the pointer has changed since the last time this method was called.
func (s *selector[T]) HasChanged() bool {
	if s.hasChanged {
		s.hasChanged = false
		return true
	} else {
		return false
	}
}

func newSelector[T any](numColumns, numRows int) selector[T] {
	s := make([][]*T, numRows)

	for i := range s {
		s[i] = make([]*T, numColumns)
	}

	return selector[T]{
		selectableItems: s,
		hasChanged:      true,
	}
}
