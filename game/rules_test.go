package game

import "testing"

func TestValidMovesPhase1_EmptyBoard(t *testing.T) {
	sc := NewScorecard()
	locked := make(map[Color]string)

	// White sum of 7 should be markable on all 4 rows
	moves := ValidMovesPhase1(sc, 7, locked)

	if len(moves) != 4 {
		t.Errorf("expected 4 valid moves for white sum 7 on empty board, got %d", len(moves))
	}

	// Each color should have number 7
	for _, m := range moves {
		if m.Number != 7 {
			t.Errorf("expected number 7, got %d", m.Number)
		}
	}
}

func TestValidMovesPhase1_LeftToRightRule(t *testing.T) {
	sc := NewScorecard()
	locked := make(map[Color]string)

	// Mark Red 8, then try white sum of 5
	sc.Mark(Red, 8)

	moves := ValidMovesPhase1(sc, 5, locked)

	// Red 5 should NOT be valid (5 is left of 8 in ascending row)
	for _, m := range moves {
		if m.Color == Red {
			t.Error("should not be able to mark Red 5 when Red 8 is already marked")
		}
	}
}

func TestValidMovesPhase1_LeftToRightRuleDescending(t *testing.T) {
	sc := NewScorecard()
	locked := make(map[Color]string)

	// Green row: 12,11,10,9,8,7,6,5,4,3,2
	// Mark Green 8, then try white sum of 10
	sc.Mark(Green, 8)

	moves := ValidMovesPhase1(sc, 10, locked)

	// Green 10 should NOT be valid (10 is to the LEFT of 8 in descending row)
	for _, m := range moves {
		if m.Color == Green {
			t.Error("should not be able to mark Green 10 when Green 8 is already marked")
		}
	}
}

func TestValidMovesPhase1_LockedRow(t *testing.T) {
	sc := NewScorecard()
	locked := map[Color]string{Red: "other_player"}

	moves := ValidMovesPhase1(sc, 7, locked)

	for _, m := range moves {
		if m.Color == Red {
			t.Error("should not be able to mark on a locked row")
		}
	}

	// Should have 3 valid moves (Yellow, Green, Blue)
	if len(moves) != 3 {
		t.Errorf("expected 3 valid moves with Red locked, got %d", len(moves))
	}
}

func TestValidMovesPhase1_RightmostRequires5Marks(t *testing.T) {
	sc := NewScorecard()
	locked := make(map[Color]string)

	// White sum of 12 on Red - rightmost number, but no marks yet
	moves := ValidMovesPhase1(sc, 12, locked)

	for _, m := range moves {
		if m.Color == Red {
			t.Error("should not be able to mark rightmost number (Red 12) without 5 marks")
		}
	}

	// Mark 5 numbers on Red
	sc.Mark(Red, 2)
	sc.Mark(Red, 3)
	sc.Mark(Red, 4)
	sc.Mark(Red, 5)
	sc.Mark(Red, 6)

	moves = ValidMovesPhase1(sc, 12, locked)

	found := false
	for _, m := range moves {
		if m.Color == Red && m.Number == 12 {
			found = true
		}
	}
	if !found {
		t.Error("should be able to mark Red 12 with 5 marks already")
	}
}

func TestValidMovesPhase1_AlreadyMarked(t *testing.T) {
	sc := NewScorecard()
	locked := make(map[Color]string)

	sc.Mark(Red, 7)

	moves := ValidMovesPhase1(sc, 7, locked)
	for _, m := range moves {
		if m.Color == Red {
			t.Error("should not be able to mark Red 7 when already marked")
		}
	}
}

func TestValidMovesPhase2(t *testing.T) {
	sc := NewScorecard()
	locked := make(map[Color]string)

	roll := &DiceRoll{
		White1:       3,
		White2:       4,
		Red:          5,
		Yellow:       2,
		Green:        6,
		Blue:         1,
		ActiveColors: map[Color]bool{Red: true, Yellow: true, Green: true, Blue: true},
	}

	moves := ValidMovesPhase2(sc, roll, locked)

	// Red: 3+5=8 or 4+5=9
	// Yellow: 3+2=5 or 4+2=6
	// Green: 3+6=9 or 4+6=10
	// Blue: 3+1=4 or 4+1=5
	// All should be valid on empty board (none are rightmost without 5 marks)
	if len(moves) != 8 {
		t.Errorf("expected 8 valid moves, got %d", len(moves))
		for _, m := range moves {
			t.Logf("  %s %d", m.Color, m.Number)
		}
	}
}

func TestValidMovesPhase2_LockedRow(t *testing.T) {
	sc := NewScorecard()
	locked := map[Color]string{Red: "someone"}

	roll := &DiceRoll{
		White1:       3,
		White2:       4,
		Red:          5,
		Yellow:       2,
		Green:        6,
		Blue:         1,
		ActiveColors: map[Color]bool{Yellow: true, Green: true, Blue: true},
	}

	moves := ValidMovesPhase2(sc, roll, locked)

	for _, m := range moves {
		if m.Color == Red {
			t.Error("should not have Red moves when Red is locked")
		}
	}
}

func TestShouldLockRow(t *testing.T) {
	sc := NewScorecard()
	sc.Mark(Red, 2)
	sc.Mark(Red, 3)
	sc.Mark(Red, 4)
	sc.Mark(Red, 5)
	sc.Mark(Red, 6)
	sc.Mark(Red, 12)

	// 12 is the rightmost for Red, and there are 6 marks (>= 5 before marking 12)
	// But ShouldLockRow checks MarkCount which includes 12 now
	if !ShouldLockRow(sc, Red, 12) {
		t.Error("should lock Red row with 6 marks including 12")
	}

	if ShouldLockRow(sc, Red, 6) {
		t.Error("should not lock Red row for non-rightmost number")
	}
}

func TestShouldLockRow_NotEnoughMarks(t *testing.T) {
	sc := NewScorecard()
	sc.Mark(Red, 2)
	sc.Mark(Red, 12)

	// Only 2 marks, shouldn't lock even with rightmost marked
	if ShouldLockRow(sc, Red, 12) {
		t.Error("should not lock with only 2 marks")
	}
}

func TestIsValidMove(t *testing.T) {
	sc := NewScorecard()
	locked := make(map[Color]string)

	if !IsValidMove(sc, Red, 5, locked) {
		t.Error("Red 5 should be valid on empty board")
	}

	sc.Mark(Red, 7)
	if IsValidMove(sc, Red, 5, locked) {
		t.Error("Red 5 should be invalid after marking Red 7")
	}
	if !IsValidMove(sc, Red, 9, locked) {
		t.Error("Red 9 should be valid after marking Red 7")
	}
}

func TestValidMovesPhase1_WhiteSum2(t *testing.T) {
	sc := NewScorecard()
	locked := make(map[Color]string)

	// White sum of 2 should be valid on Red, Yellow (first number)
	// and Green, Blue (last number = 2, needs 5 marks)
	moves := ValidMovesPhase1(sc, 2, locked)

	colorFound := make(map[Color]bool)
	for _, m := range moves {
		colorFound[m.Color] = true
	}

	if !colorFound[Red] {
		t.Error("Red 2 should be valid (first number in ascending row)")
	}
	if !colorFound[Yellow] {
		t.Error("Yellow 2 should be valid (first number in ascending row)")
	}
	if colorFound[Green] {
		t.Error("Green 2 should NOT be valid (rightmost number, needs 5 marks)")
	}
	if colorFound[Blue] {
		t.Error("Blue 2 should NOT be valid (rightmost number, needs 5 marks)")
	}
}
