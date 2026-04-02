package game

import "testing"

func TestNewScorecard(t *testing.T) {
	sc := NewScorecard()

	for _, c := range AllColors {
		if sc.MarkCount(c) != 0 {
			t.Errorf("expected 0 marks for %s, got %d", c, sc.MarkCount(c))
		}
	}

	if sc.Penalties != 0 {
		t.Errorf("expected 0 penalties, got %d", sc.Penalties)
	}
}

func TestMark(t *testing.T) {
	sc := NewScorecard()
	sc.Mark(Red, 5)

	if !sc.IsMarked(Red, 5) {
		t.Error("expected Red 5 to be marked")
	}
	if sc.IsMarked(Red, 6) {
		t.Error("expected Red 6 to not be marked")
	}
	if sc.MarkCount(Red) != 1 {
		t.Errorf("expected 1 mark on Red, got %d", sc.MarkCount(Red))
	}
}

func TestRightmostMarkedIndex(t *testing.T) {
	sc := NewScorecard()

	if sc.RightmostMarkedIndex(Red) != -1 {
		t.Error("expected -1 for empty row")
	}

	sc.Mark(Red, 3)
	sc.Mark(Red, 7)

	// Red row: 2,3,4,5,6,7,8,9,10,11,12
	// Index of 7 is 5
	if sc.RightmostMarkedIndex(Red) != 5 {
		t.Errorf("expected rightmost index 5, got %d", sc.RightmostMarkedIndex(Red))
	}
}

func TestRightmostMarkedIndexDescending(t *testing.T) {
	sc := NewScorecard()
	sc.Mark(Green, 10) // Green row: 12,11,10,...
	// Index of 10 is 2

	if sc.RightmostMarkedIndex(Green) != 2 {
		t.Errorf("expected rightmost index 2, got %d", sc.RightmostMarkedIndex(Green))
	}
}

func TestCanLock(t *testing.T) {
	sc := NewScorecard()

	// Can't lock with no marks
	if sc.CanLock(Red) {
		t.Error("should not be able to lock with 0 marks")
	}

	// Mark 5 numbers including the rightmost
	sc.Mark(Red, 2)
	sc.Mark(Red, 3)
	sc.Mark(Red, 4)
	sc.Mark(Red, 5)
	sc.Mark(Red, 6)

	// 5 marks but rightmost (12) not marked
	if sc.CanLock(Red) {
		t.Error("should not be able to lock without marking rightmost number")
	}

	// Mark the rightmost number
	sc.Mark(Red, 12)

	// Now can lock (6 marks including 12)
	if !sc.CanLock(Red) {
		t.Error("should be able to lock with 6 marks including rightmost")
	}
}

func TestCanLockGreen(t *testing.T) {
	sc := NewScorecard()

	// Green row goes 12,11,10,9,8,7,6,5,4,3,2
	// Rightmost number is 2
	sc.Mark(Green, 12)
	sc.Mark(Green, 11)
	sc.Mark(Green, 10)
	sc.Mark(Green, 9)
	sc.Mark(Green, 8)
	sc.Mark(Green, 2)

	if !sc.CanLock(Green) {
		t.Error("should be able to lock Green with 6 marks including 2")
	}
}

func TestAddPenalty(t *testing.T) {
	sc := NewScorecard()

	for i := 0; i < 3; i++ {
		if sc.AddPenalty() {
			t.Errorf("should not have 4 penalties after %d", i+1)
		}
	}

	if !sc.AddPenalty() {
		t.Error("should have 4 penalties now")
	}
}

func TestScoreRow(t *testing.T) {
	tests := []struct {
		marks  int
		locked bool
		want   int
	}{
		{0, false, 0},
		{1, false, 1},
		{2, false, 3},
		{3, false, 6},
		{4, false, 10},
		{5, false, 15},
		{5, true, 21}, // 5 marks + lock = 6 -> 21
		{6, false, 21},
		{7, false, 28},
		{8, false, 36},
		{9, false, 45},
		{10, false, 55},
		{11, false, 66},
		{12, false, 78},
	}

	for _, tt := range tests {
		got := ScoreRow(tt.marks, tt.locked)
		if got != tt.want {
			t.Errorf("ScoreRow(%d, %v) = %d, want %d", tt.marks, tt.locked, got, tt.want)
		}
	}
}

func TestTotalScore(t *testing.T) {
	sc := NewScorecard()

	// Mark some numbers
	sc.Mark(Red, 2)
	sc.Mark(Red, 3) // 2 marks = 3 pts

	sc.Mark(Yellow, 5)
	sc.Mark(Yellow, 6)
	sc.Mark(Yellow, 7) // 3 marks = 6 pts

	sc.Penalties = 2 // -10 pts

	locked := make(map[Color]string)
	total := sc.TotalScore(locked, "player1")

	// 3 + 6 + 0 + 0 - 10 = -1
	if total != -1 {
		t.Errorf("expected total -1, got %d", total)
	}
}

func TestTotalScoreWithLock(t *testing.T) {
	sc := NewScorecard()

	// Mark 5 numbers + rightmost on Red
	sc.Mark(Red, 2)
	sc.Mark(Red, 3)
	sc.Mark(Red, 4)
	sc.Mark(Red, 5)
	sc.Mark(Red, 6)
	sc.Mark(Red, 12)

	locked := map[Color]string{Red: "player1"}
	total := sc.TotalScore(locked, "player1")

	// 6 marks + lock bonus = 7 -> 28 pts
	if total != 28 {
		t.Errorf("expected total 28, got %d", total)
	}
}

func TestRowNumbers(t *testing.T) {
	redNums := RowNumbers(Red)
	if redNums[0] != 2 || redNums[10] != 12 {
		t.Error("Red row should be 2-12 ascending")
	}

	greenNums := RowNumbers(Green)
	if greenNums[0] != 12 || greenNums[10] != 2 {
		t.Error("Green row should be 12-2 descending")
	}
}
