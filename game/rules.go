package game

// Move represents a player's move: marking a number on a row.
type Move struct {
	Color  Color
	Number int
}

// ValidMovesPhase1 returns all valid moves for a player during Phase 1
// (using the white dice sum). A player can mark the white sum on any
// non-locked row, as long as it respects the left-to-right rule.
func ValidMovesPhase1(sc *Scorecard, whiteSum int, lockedRows map[Color]string) []Move {
	var moves []Move
	for _, c := range AllColors {
		if _, locked := lockedRows[c]; locked {
			continue
		}
		if canMark(sc, c, whiteSum, lockedRows) {
			moves = append(moves, Move{Color: c, Number: whiteSum})
		}
	}
	return moves
}

// ValidMovesPhase2 returns all valid moves for the active player during Phase 2
// (using a white die + colored die combo).
func ValidMovesPhase2(sc *Scorecard, roll *DiceRoll, lockedRows map[Color]string) []Move {
	var moves []Move
	seen := make(map[Move]bool)

	combos := roll.ColorCombos()
	for _, combo := range combos {
		if _, locked := lockedRows[combo.Color]; locked {
			continue
		}
		m := Move{Color: combo.Color, Number: combo.Sum}
		if seen[m] {
			continue
		}
		if canMark(sc, combo.Color, combo.Sum, lockedRows) {
			moves = append(moves, m)
			seen[m] = true
		}
	}
	return moves
}

// canMark checks if a number can be marked on a row, respecting:
// 1. The number must be in the row's valid range (2-12)
// 2. The number must not already be marked
// 3. The number must be to the right of the rightmost marked number (left-to-right rule)
// 4. To mark the rightmost number (lock position), the player must have at least 5 marks
func canMark(sc *Scorecard, c Color, number int, lockedRows map[Color]string) bool {
	nums := RowNumbers(c)

	// Find the index of the target number in the row
	targetIdx := -1
	for i, n := range nums {
		if n == number {
			targetIdx = i
			break
		}
	}
	if targetIdx == -1 {
		return false // Number not in this row
	}

	// Number must not already be marked
	if sc.IsMarked(c, number) {
		return false
	}

	// Must be to the right of the rightmost marked number
	rightmostIdx := sc.RightmostMarkedIndex(c)
	if targetIdx <= rightmostIdx {
		return false
	}

	// Special rule: to mark the rightmost number (lock position),
	// the player must have at least 5 marks in the row already.
	if number == RightmostNumber(c) && sc.MarkCount(c) < 5 {
		return false
	}

	return true
}

// IsValidMove checks if a specific move is valid for a player.
func IsValidMove(sc *Scorecard, c Color, number int, lockedRows map[Color]string) bool {
	if _, locked := lockedRows[c]; locked {
		return false
	}
	return canMark(sc, c, number, lockedRows)
}

// ShouldLockRow checks if marking this number should trigger a row lock.
// A row is locked when a player marks the rightmost number with at least 5 other marks.
func ShouldLockRow(sc *Scorecard, c Color, number int) bool {
	return number == RightmostNumber(c) && sc.MarkCount(c) >= 5
}
