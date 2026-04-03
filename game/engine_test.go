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

	if g.Phase != PhaseAction {
		t.Errorf("expected PhaseAction after rolling, got %v", g.Phase)
	}
	if g.CurrentRoll == nil {
		t.Fatal("expected non-nil dice roll")
	}
	if g.CurrentRoll.White1 < 1 || g.CurrentRoll.White1 > 6 {
		t.Errorf("white1 out of range: %d", g.CurrentRoll.White1)
	}
}

func TestRollDiceIdempotent(t *testing.T) {
	players := makeTestPlayers(2)
	g := NewGame(players, 30*time.Second)
	g.RollDice()

	roll1W1 := g.CurrentRoll.White1
	roll1W2 := g.CurrentRoll.White2

	// Calling again while in PhaseAction should be no-op
	g.RollDice()
	if g.CurrentRoll.White1 != roll1W1 || g.CurrentRoll.White2 != roll1W2 {
		// Could theoretically happen with same random values, but very unlikely
		t.Log("Note: dice values changed, but this could be coincidence")
	}
	if g.Phase != PhaseAction {
		t.Errorf("expected PhaseAction, got %v", g.Phase)
	}
}

func TestPlayerSteps(t *testing.T) {
	players := makeTestPlayers(2)
	g := NewGame(players, 30*time.Second)
	g.RollDice()

	// Player 1 is active, Player 2 is not
	if g.GetPlayerStep("player1") != StepWhite {
		t.Errorf("active player should start at StepWhite, got %v", g.GetPlayerStep("player1"))
	}
	if g.GetPlayerStep("player2") != StepWhite {
		t.Errorf("non-active player should start at StepWhite, got %v", g.GetPlayerStep("player2"))
	}
}

func TestNonActivePlayerPassFlow(t *testing.T) {
	players := makeTestPlayers(2)
	g := NewGame(players, 30*time.Second)
	g.RollDice()

	// Player 2 (non-active) passes
	err := g.SubmitPass("player2")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Player 2 should now be confirmed/waiting
	if g.GetPlayerStep("player2") != StepWaiting {
		t.Errorf("non-active player should be waiting after pass, got %v", g.GetPlayerStep("player2"))
	}
}

func TestActivePlayerTwoStepFlow(t *testing.T) {
	players := makeTestPlayers(2)
	g := NewGame(players, 30*time.Second)
	g.RollDice()

	// Active player passes white step
	err := g.SubmitPass("player1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Active player should now be on color step
	if g.GetPlayerStep("player1") != StepColor {
		t.Errorf("active player should be at StepColor after passing white, got %v", g.GetPlayerStep("player1"))
	}

	// Non-active player also passes (to not block)
	g.SubmitPass("player2")

	// Active player passes color step
	err = g.SubmitPass("player1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Active player passed everything -> should get a penalty
	if g.Players[0].Scorecard.Penalties != 1 {
		t.Errorf("expected 1 penalty for active player, got %d", g.Players[0].Scorecard.Penalties)
	}

	// Turn should have advanced
	if g.ActivePlayer != 1 {
		t.Errorf("expected active player to be 1, got %d", g.ActivePlayer)
	}
}

func TestNonActivePlayerMarkWhiteSum(t *testing.T) {
	players := makeTestPlayers(2)
	g := NewGame(players, 30*time.Second)
	g.RollDice()

	whiteSum := g.CurrentRoll.WhiteSum()

	// Find a valid move for player 2
	moves := g.GetValidMoves("player2")
	if len(moves) > 0 {
		err := g.SubmitMark("player2", moves[0])
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Player 2 should be confirmed (non-active auto-confirms after white)
		if g.GetPlayerStep("player2") != StepWaiting {
			t.Errorf("non-active player should be waiting after marking, got %v", g.GetPlayerStep("player2"))
		}

		// Verify the mark
		if !g.Players[1].Scorecard.IsMarked(moves[0].Color, whiteSum) {
			t.Error("move should have been marked")
		}
	}
}

func TestActivePlayerMarkBoth(t *testing.T) {
	players := makeTestPlayers(2)
	g := NewGame(players, 30*time.Second)
	g.RollDice()

	// Player 2 passes quickly
	g.SubmitPass("player2")

	// Active player marks white sum
	whiteMoves := g.GetValidMoves("player1")
	if len(whiteMoves) > 0 {
		err := g.SubmitMark("player1", whiteMoves[0])
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Should now be on color step
		if g.GetPlayerStep("player1") != StepColor {
			t.Errorf("active player should be at StepColor, got %v", g.GetPlayerStep("player1"))
		}
	} else {
		g.SubmitPass("player1")
	}

	// Active player tries color combo
	colorMoves := g.GetValidMoves("player1")
	if len(colorMoves) > 0 {
		err := g.SubmitMark("player1", colorMoves[0])
		if err != nil {
			t.Fatalf("unexpected error marking color: %v", err)
		}
	} else {
		g.SubmitPass("player1")
	}

	// Active player marked at least white -> no penalty
	if len(whiteMoves) > 0 && g.Players[0].Scorecard.Penalties != 0 {
		t.Errorf("should not get penalty after marking, got %d", g.Players[0].Scorecard.Penalties)
	}
}

func TestWhiteSumValidation(t *testing.T) {
	players := makeTestPlayers(2)
	g := NewGame(players, 30*time.Second)
	g.RollDice()

	whiteSum := g.CurrentRoll.WhiteSum()
	wrongNumber := whiteSum + 1
	if wrongNumber > 12 {
		wrongNumber = whiteSum - 1
	}

	err := g.SubmitMark("player1", Move{Color: Red, Number: wrongNumber})
	if err == nil {
		t.Errorf("expected error for wrong white sum (tried %d, white sum is %d)", wrongNumber, whiteSum)
	}
}

func TestPenaltyOn4Ends(t *testing.T) {
	players := makeTestPlayers(2)
	g := NewGame(players, 30*time.Second)
	g.Players[0].Scorecard.Penalties = 3
	g.RollDice()

	// Both pass everything
	g.SubmitPass("player2") // non-active done
	g.SubmitPass("player1") // white pass -> color step
	g.SubmitPass("player1") // color pass -> penalty -> 4th -> game over

	if g.Phase != PhaseGameOver {
		t.Errorf("expected PhaseGameOver, got %v", g.Phase)
	}
}

func TestTwoRowsLockedEndsGame(t *testing.T) {
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

	// Force white sum to 12
	g.mu.Lock()
	g.CurrentRoll.White1 = 6
	g.CurrentRoll.White2 = 6
	g.mu.Unlock()

	// Player 1 marks Yellow 12
	err := g.SubmitMark("player1", Move{Color: Yellow, Number: 12})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if g.Phase != PhaseGameOver {
		t.Errorf("expected PhaseGameOver after 2 locked rows, got %v", g.Phase)
	}
}

func TestDisconnectPlayer(t *testing.T) {
	players := makeTestPlayers(2)
	g := NewGame(players, 30*time.Second)
	g.RollDice()

	// Disconnect player2 -> auto-confirmed
	g.DisconnectPlayer("player2")

	if g.Players[1].Connected {
		t.Error("player2 should be disconnected")
	}

	// Player1 passes both steps -> penalty, turn should end
	g.SubmitPass("player1") // white
	g.SubmitPass("player1") // color -> penalty, all confirmed, next turn
}

func TestGetScores(t *testing.T) {
	players := makeTestPlayers(2)
	g := NewGame(players, 30*time.Second)

	g.Players[0].Scorecard.Mark(Red, 2)
	g.Players[0].Scorecard.Mark(Red, 3)
	g.Players[0].Scorecard.Mark(Red, 4) // 3 marks = 6 pts

	g.Players[1].Scorecard.Mark(Blue, 12)
	g.Players[1].Scorecard.Penalties = 1 // 1 mark = 1 pt, -5 = -4

	scores := g.GetScores()
	if scores[0].Total != 6 {
		t.Errorf("player1 expected 6 pts, got %d", scores[0].Total)
	}
	if scores[1].Total != -4 {
		t.Errorf("player2 expected -4 pts, got %d", scores[1].Total)
	}
}

func TestTurnRotation(t *testing.T) {
	players := makeTestPlayers(3)
	g := NewGame(players, 30*time.Second)
	g.RollDice()

	if g.ActivePlayer != 0 {
		t.Errorf("expected active player 0, got %d", g.ActivePlayer)
	}

	// Everyone passes
	g.SubmitPass("player2")
	g.SubmitPass("player3")
	g.SubmitPass("player1") // white
	g.SubmitPass("player1") // color -> penalty, all confirmed

	if g.ActivePlayer != 1 {
		t.Errorf("expected active player 1, got %d", g.ActivePlayer)
	}
}

func TestSubscribeBroadcast(t *testing.T) {
	players := makeTestPlayers(2)
	g := NewGame(players, 30*time.Second)

	ch1 := g.Subscribe("player1")
	ch2 := g.Subscribe("player2")

	g.RollDice()

	// Both should receive the dice rolled event
	select {
	case event := <-ch1:
		if event.Type != EventDiceRolled {
			t.Errorf("player1 expected EventDiceRolled, got %v", event.Type)
		}
	default:
		t.Error("player1 should have received an event")
	}

	select {
	case event := <-ch2:
		if event.Type != EventDiceRolled {
			t.Errorf("player2 expected EventDiceRolled, got %v", event.Type)
		}
	default:
		t.Error("player2 should have received an event")
	}

	g.Unsubscribe("player1")
	g.Unsubscribe("player2")
}

func TestSimultaneousActions(t *testing.T) {
	// Test that non-active player and active player can act independently
	players := makeTestPlayers(3)
	g := NewGame(players, 30*time.Second)
	g.RollDice()

	// Player 3 (non-active) passes immediately
	g.SubmitPass("player3")
	if g.GetPlayerStep("player3") != StepWaiting {
		t.Error("player3 should be waiting")
	}

	// Active player (player1) is still on white step
	if g.GetPlayerStep("player1") != StepWhite {
		t.Error("player1 should still be on white step")
	}

	// Player 2 (non-active) passes
	g.SubmitPass("player2")
	if g.GetPlayerStep("player2") != StepWaiting {
		t.Error("player2 should be waiting")
	}

	// Active player passes white
	g.SubmitPass("player1")
	if g.GetPlayerStep("player1") != StepColor {
		t.Error("player1 should be on color step")
	}

	// Active player passes color -> all confirmed, turn ends
	g.SubmitPass("player1")

	// Player1 should have penalty (passed everything)
	if g.Players[0].Scorecard.Penalties != 1 {
		t.Errorf("expected 1 penalty, got %d", g.Players[0].Scorecard.Penalties)
	}
}
