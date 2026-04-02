package game

// Color represents a row color in Qwixx.
type Color int

const (
	Red Color = iota
	Yellow
	Green
	Blue
)

var AllColors = []Color{Red, Yellow, Green, Blue}

func (c Color) String() string {
	switch c {
	case Red:
		return "Red"
	case Yellow:
		return "Yellow"
	case Green:
		return "Green"
	case Blue:
		return "Blue"
	default:
		return "Unknown"
	}
}

// RowNumbers returns the ordered sequence of numbers for a given color row.
// Red and Yellow are ascending (2-12), Green and Blue are descending (12-2).
func RowNumbers(c Color) []int {
	switch c {
	case Red, Yellow:
		return []int{2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12}
	case Green, Blue:
		return []int{12, 11, 10, 9, 8, 7, 6, 5, 4, 3, 2}
	default:
		return nil
	}
}

// RightmostNumber returns the last number in a row (the lock target).
// Red/Yellow: 12, Green/Blue: 2.
func RightmostNumber(c Color) int {
	nums := RowNumbers(c)
	return nums[len(nums)-1]
}

// Scorecard represents a single player's scorecard.
type Scorecard struct {
	// Marks tracks which numbers are marked on each row.
	// Key is color, value is set of marked numbers.
	Marks map[Color]map[int]bool

	// Penalties is the number of penalties taken (max 4).
	Penalties int
}

// NewScorecard creates a fresh scorecard with no marks.
func NewScorecard() *Scorecard {
	marks := make(map[Color]map[int]bool)
	for _, c := range AllColors {
		marks[c] = make(map[int]bool)
	}
	return &Scorecard{
		Marks:     marks,
		Penalties: 0,
	}
}

// MarkCount returns the number of marks in a given row.
func (s *Scorecard) MarkCount(c Color) int {
	return len(s.Marks[c])
}

// IsMarked returns true if the given number is marked on the given row.
func (s *Scorecard) IsMarked(c Color, number int) bool {
	return s.Marks[c][number]
}

// Mark marks a number on the given row. Does not validate the move.
func (s *Scorecard) Mark(c Color, number int) {
	s.Marks[c][number] = true
}

// RightmostMarkedIndex returns the index of the rightmost marked number in a row,
// or -1 if no numbers are marked.
func (s *Scorecard) RightmostMarkedIndex(c Color) int {
	nums := RowNumbers(c)
	rightmost := -1
	for i, n := range nums {
		if s.Marks[c][n] {
			rightmost = i
		}
	}
	return rightmost
}

// CanLock returns true if the player can lock this row
// (has at least 5 marks and is marking the rightmost number).
func (s *Scorecard) CanLock(c Color) bool {
	return s.MarkCount(c) >= 5 && s.IsMarked(c, RightmostNumber(c))
}

// AddPenalty adds a penalty to the scorecard. Returns true if the player now has 4 penalties.
func (s *Scorecard) AddPenalty() bool {
	s.Penalties++
	return s.Penalties >= 4
}

// ScoreRow calculates the score for a single row using triangular scoring.
// n marks = n*(n+1)/2 points.
// If the row is locked by this player, the lock counts as an extra mark.
func ScoreRow(markCount int, locked bool) int {
	n := markCount
	if locked {
		n++ // The lock mark counts as an additional mark
	}
	return n * (n + 1) / 2
}

// TotalScore calculates the total score for a scorecard.
// lockedBy indicates which player locked each row (playerID or "" if not locked).
// playerID is the ID of this scorecard's player.
func (s *Scorecard) TotalScore(lockedRows map[Color]string, playerID string) int {
	total := 0
	for _, c := range AllColors {
		locked := lockedRows[c] == playerID
		total += ScoreRow(s.MarkCount(c), locked)
	}
	total -= s.Penalties * 5
	return total
}

// RowScore returns the score for a single row of this scorecard.
func (s *Scorecard) RowScore(c Color, lockedRows map[Color]string, playerID string) int {
	locked := lockedRows[c] == playerID
	return ScoreRow(s.MarkCount(c), locked)
}
