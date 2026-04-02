package game

import (
	"fmt"
	"testing"
	"time"
)

func makeTestPlayers(n int) []*PlayerState {
	players := make([]*PlayerState, n)
	for i := 0; i < n; i++ {
		players[i] = &PlayerState{
			ID:        fmt.Sprintf("player%d", i+1),
			Nickname:  fmt.Sprintf("Player%d", i+1),
			Connected: true,
		}
	}
	return players
}

func TestNewGame(t *testing.T) {
	players := makeTestPlayers(3)
	g := NewGame(players, 30*time.Second)

	if g.Phase != PhaseRolling {
		t.Errorf("expected PhaseRolling, got %v", g.Phase)
	}

	if g.ActivePlayer != 0 {
		t.Errorf("expected active player 0, got %d", g.ActivePlayer)
	}

	for _, p := range g.Players {
		if p.Scorecard == nil {
			t.Errorf("player %s has nil scorecard", p.Nickname)
		}
	}
}

func TestRollDice(t *testing.T) {
	players := makeTestPlayers(2)
	g := NewGame(players, 30*time.Second)

	g.RollDice()

	if g.Phase != PhaseWhiteSum {
		t.Errorf("expected PhaseWhiteSum after rolling, got %v", g.Phase)
	}

	if g.CurrentRoll == nil {
		t.Fatal("expected non-nil dice roll")
	}

	// All dice values should be 1-6
	if g.CurrentRoll.White1 < 1 || g.CurrentRoll.White1 > 6 {
		t.Errorf("white1 out of range: %d", g.CurrentRoll.White1)
	}
	if g.CurrentRoll.White2 < 1 || g.CurrentRoll.White2 > 6 {
		t.Errorf("white2 out of range: %d", g.CurrentRoll.White2)
	}
}

func TestPhase1AllPass(t *testing.T) {
	players := makeTestPlayers(2)
	g := NewGame(players, 30*time.Second)
	g.RollDice()

	// Both players pass
	err := g.SubmitPhase1Move("player1", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// After first player passes, should still be Phase 1
	if g.Phase != PhaseWhiteSum {
		t.Errorf("expected PhaseWhiteSum, got %v", g.Phase)
	}

	err = g.SubmitPhase1Move("player2", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// After all pass, should transition to Phase 2
	if g.Phase != PhaseColorCombo {
		t.Errorf("expected PhaseColorCombo, got %v", g.Phase)
	}
}

func TestPhase1Mark(t *testing.T) {
	players := makeTestPlayers(2)
	g := NewGame(players, 30*time.Second)
	g.RollDice()

	whiteSum := g.CurrentRoll.WhiteSum()

	// Player 1 marks if possible
	moves := g.GetValidMovesPhase1("player1")
	if len(moves) > 0 {
		err := g.SubmitPhase1Move("player1", &moves[0])
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Verify the mark
		sc := g.Players[0].Scorecard
		if !sc.IsMarked(moves[0].Color, whiteSum) {
			t.Error("move should have been marked on scorecard")
		}
	}
}

func TestPhase1DuplicateAction(t *testing.T) {
	players := makeTestPlayers(2)
	g := NewGame(players, 30*time.Second)
	g.RollDice()

	g.SubmitPhase1Move("player1", nil)
	err := g.SubmitPhase1Move("player1", nil)

	if err == nil {
		t.Error("expected error for duplicate action")
	}
}

func TestPhase2PenaltyForNoMarks(t *testing.T) {
	players := makeTestPlayers(2)
	g := NewGame(players, 30*time.Second)
	g.RollDice()

	// Both pass Phase 1
	g.SubmitPhase1Move("player1", nil)
	g.SubmitPhase1Move("player2", nil)

	// Active player (player1) passes Phase 2 too
	err := g.SubmitPhase2Move("player1", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Player1 should have a penalty (passed both phases)
	if g.Players[0].Scorecard.Penalties != 1 {
		t.Errorf("expected 1 penalty, got %d", g.Players[0].Scorecard.Penalties)
	}

	// Should have advanced to next turn (PhaseRolling, active player 1)
	if g.Phase != PhaseRolling {
		t.Errorf("expected PhaseRolling, got %v", g.Phase)
	}
	if g.ActivePlayer != 1 {
		t.Errorf("expected active player 1, got %d", g.ActivePlayer)
	}
}

func TestPhase2NoPenaltyIfMarkedInPhase1(t *testing.T) {
	players := makeTestPlayers(2)
	g := NewGame(players, 30*time.Second)
	g.RollDice()

	// Get valid moves for the active player (player1)
	moves := g.GetValidMovesPhase1("player1")

	if len(moves) > 0 {
		// Player 1 marks in Phase 1
		g.SubmitPhase1Move("player1", &moves[0])
		g.SubmitPhase1Move("player2", nil)

		// Player 1 passes Phase 2 - should NOT get a penalty
		g.SubmitPhase2Move("player1", nil)

		if g.Players[0].Scorecard.Penalties != 0 {
			t.Errorf("should not get penalty after marking in Phase 1, got %d penalties", g.Players[0].Scorecard.Penalties)
		}
	}
}

func TestGameEnd4Penalties(t *testing.T) {
	players := makeTestPlayers(2)
	g := NewGame(players, 30*time.Second)

	// Give player1 3 penalties manually
	g.Players[0].Scorecard.Penalties = 3

	g.RollDice()

	// Both pass Phase 1
	g.SubmitPhase1Move("player1", nil)
	g.SubmitPhase1Move("player2", nil)

	// Player1 passes Phase 2 -> 4th penalty -> game over
	g.SubmitPhase2Move("player1", nil)

	if g.Phase != PhaseGameOver {
		t.Errorf("expected PhaseGameOver, got %v", g.Phase)
	}
}

func TestGameEnd2RowsLocked(t *testing.T) {
	players := makeTestPlayers(2)
	g := NewGame(players, 30*time.Second)

	// Pre-lock one row
	g.LockedRows[Red] = "player1"

	// Set up player1's Yellow row with 5 marks
	sc := g.Players[0].Scorecard
	sc.Mark(Yellow, 2)
	sc.Mark(Yellow, 3)
	sc.Mark(Yellow, 4)
	sc.Mark(Yellow, 5)
	sc.Mark(Yellow, 6)

	g.RollDice()

	// Force white sum to 12 for testing
	g.mu.Lock()
	g.CurrentRoll.White1 = 6
	g.CurrentRoll.White2 = 6
	g.mu.Unlock()

	// Player1 marks Yellow 12 (should trigger lock)
	move := &Move{Color: Yellow, Number: 12}
	err := g.SubmitPhase1Move("player1", move)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if g.Phase != PhaseGameOver {
		t.Errorf("expected PhaseGameOver after 2 locked rows, got %v", g.Phase)
	}
}

func TestAutoPassPhase1(t *testing.T) {
	players := makeTestPlayers(3)
	g := NewGame(players, 30*time.Second)
	g.RollDice()

	// Only player1 acts
	g.SubmitPhase1Move("player1", nil)

	// Auto-pass the rest
	g.AutoPassPhase1()

	if g.Phase != PhaseColorCombo {
		t.Errorf("expected PhaseColorCombo after auto-pass, got %v", g.Phase)
	}
}

func TestAutoPassPhase2(t *testing.T) {
	players := makeTestPlayers(2)
	g := NewGame(players, 30*time.Second)
	g.RollDice()

	g.SubmitPhase1Move("player1", nil)
	g.SubmitPhase1Move("player2", nil)

	// Auto-pass phase 2 for active player
	g.AutoPassPhase2()

	// Should get a penalty (passed both phases)
	if g.Players[0].Scorecard.Penalties != 1 {
		t.Errorf("expected 1 penalty after auto-pass, got %d", g.Players[0].Scorecard.Penalties)
	}

	if g.Phase != PhaseRolling {
		t.Errorf("expected PhaseRolling after auto-pass phase 2, got %v", g.Phase)
	}
}

func TestDisconnectPlayer(t *testing.T) {
	players := makeTestPlayers(2)
	g := NewGame(players, 30*time.Second)
	g.RollDice()

	// Disconnect player2 during Phase 1
	g.DisconnectPlayer("player2")

	if g.Players[1].Connected {
		t.Error("player2 should be disconnected")
	}

	// Player1 passes -> should transition (player2 auto-counted as acted)
	g.SubmitPhase1Move("player1", nil)

	if g.Phase != PhaseColorCombo {
		t.Errorf("expected PhaseColorCombo, got %v", g.Phase)
	}
}

func TestGetScores(t *testing.T) {
	players := makeTestPlayers(2)
	g := NewGame(players, 30*time.Second)

	g.Players[0].Scorecard.Mark(Red, 2)
	g.Players[0].Scorecard.Mark(Red, 3)
	g.Players[0].Scorecard.Mark(Red, 4) // 3 marks = 6 pts

	g.Players[1].Scorecard.Mark(Blue, 12)
	g.Players[1].Scorecard.Penalties = 1 // 1 mark = 1 pt, -5 penalty = -4 pts

	scores := g.GetScores()

	if scores[0].Total != 6 {
		t.Errorf("player1 expected 6 points, got %d", scores[0].Total)
	}
	if scores[1].Total != -4 {
		t.Errorf("player2 expected -4 points, got %d", scores[1].Total)
	}
}

func TestTurnRotation(t *testing.T) {
	players := makeTestPlayers(3)
	g := NewGame(players, 30*time.Second)

	if g.ActivePlayer != 0 {
		t.Errorf("expected first active player to be 0, got %d", g.ActivePlayer)
	}

	// Complete a full turn
	g.RollDice()
	g.SubmitPhase1Move("player1", nil)
	g.SubmitPhase1Move("player2", nil)
	g.SubmitPhase1Move("player3", nil)
	g.SubmitPhase2Move("player1", nil) // penalty

	if g.ActivePlayer != 1 {
		t.Errorf("expected active player 1 after turn, got %d", g.ActivePlayer)
	}
}
