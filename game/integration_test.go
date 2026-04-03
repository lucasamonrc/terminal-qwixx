package game

import (
	"testing"
	"time"
)

// TestFullGamePassOnly simulates a complete 2-player game where everyone passes.
func TestFullGamePassOnly(t *testing.T) {
	players := makeTestPlayers(2)
	g := NewGame(players, 30*time.Second)

	ch1 := g.Subscribe("player1")
	ch2 := g.Subscribe("player2")

	g.StartGameLoop()
	defer g.StopGameLoop()

	time.Sleep(300 * time.Millisecond)

	turnCount := 0
	maxTurns := 20

	for turnCount < maxTurns {
		if g.GetPhase() == PhaseGameOver {
			break
		}

		if g.GetPhase() != PhaseAction {
			time.Sleep(300 * time.Millisecond)
			continue
		}

		// Non-active player passes
		activeID := g.GetActivePlayerID()
		nonActiveID := "player1"
		if activeID == "player1" {
			nonActiveID = "player2"
		}

		g.SubmitPass(nonActiveID)

		// Active player passes both steps
		g.SubmitPass(activeID) // white
		g.SubmitPass(activeID) // color -> penalty, turn ends

		turnCount++
		time.Sleep(500 * time.Millisecond) // Wait for next roll
	}

	if g.GetPhase() != PhaseGameOver {
		t.Fatalf("game should have ended after %d turns, phase is %v", turnCount, g.GetPhase())
	}

	scores := g.GetScores()
	foundPenalized := false
	for _, s := range scores {
		if s.Penalties >= 4 {
			foundPenalized = true
		}
		t.Logf("%s: %d penalties, %d pts", s.Nickname, s.Penalties, s.Total)
	}
	if !foundPenalized {
		t.Error("expected one player to have 4 penalties")
	}

	p1Events := drainEvents(ch1)
	p2Events := drainEvents(ch2)
	t.Logf("Player1 received %d events, Player2 received %d events", len(p1Events), len(p2Events))

	if len(p1Events) == 0 || len(p2Events) == 0 {
		t.Error("both players should have received events")
	}

	g.Unsubscribe("player1")
	g.Unsubscribe("player2")
}

// TestFullGameWithMarks simulates a game where players make actual marks.
func TestFullGameWithMarks(t *testing.T) {
	players := makeTestPlayers(3)
	g := NewGame(players, 30*time.Second)

	g.Subscribe("player1")
	g.Subscribe("player2")
	g.Subscribe("player3")

	g.StartGameLoop()
	defer g.StopGameLoop()

	time.Sleep(300 * time.Millisecond)

	turnCount := 0
	maxTurns := 40

	for turnCount < maxTurns {
		if g.GetPhase() == PhaseGameOver {
			break
		}

		if g.GetPhase() != PhaseAction {
			time.Sleep(300 * time.Millisecond)
			continue
		}

		activeID := g.GetActivePlayerID()

		// All players try to mark or pass on white step
		for _, id := range []string{"player1", "player2", "player3"} {
			moves := g.GetValidMoves(id)
			if len(moves) > 0 {
				g.SubmitMark(id, moves[0])
			} else {
				g.SubmitPass(id)
			}
		}

		// Active player: try color combo
		if g.GetPhase() == PhaseAction { // Game might have ended from a lock
			colorMoves := g.GetValidMoves(activeID)
			if len(colorMoves) > 0 {
				g.SubmitMark(activeID, colorMoves[0])
			} else {
				g.SubmitPass(activeID)
			}
		}

		turnCount++
		time.Sleep(500 * time.Millisecond)
	}

	if g.GetPhase() != PhaseGameOver {
		t.Fatalf("game should have ended, phase is %v after %d turns", g.GetPhase(), turnCount)
	}

	scores := g.GetScores()
	for _, s := range scores {
		t.Logf("%s: Red=%d Yel=%d Grn=%d Blu=%d Pen=%d Total=%d",
			s.Nickname, s.RowScores[Red], s.RowScores[Yellow],
			s.RowScores[Green], s.RowScores[Blue], s.Penalties, s.Total)
	}
	t.Logf("Game over: %s (after %d turns)", g.GetGameOverReason(), turnCount)

	g.Unsubscribe("player1")
	g.Unsubscribe("player2")
	g.Unsubscribe("player3")
}

// TestGameLoopAutoRolls verifies dice auto-roll after turn ends.
func TestGameLoopAutoRolls(t *testing.T) {
	players := makeTestPlayers(2)
	g := NewGame(players, 30*time.Second)

	g.Subscribe("player1")
	g.Subscribe("player2")

	g.StartGameLoop()
	defer g.StopGameLoop()

	time.Sleep(300 * time.Millisecond)

	if g.GetPhase() != PhaseAction {
		t.Fatalf("expected PhaseAction after start, got %v", g.GetPhase())
	}

	// Both pass everything
	g.SubmitPass("player2")
	g.SubmitPass("player1") // white
	g.SubmitPass("player1") // color -> penalty, turn ends -> PhaseRolling

	// Wait for game loop to auto-roll
	time.Sleep(500 * time.Millisecond)

	if g.GetPhase() != PhaseAction {
		t.Errorf("expected PhaseAction after auto-roll, got %v", g.GetPhase())
	}

	if g.GetActivePlayerID() != "player2" {
		t.Errorf("expected player2's turn, got %s", g.GetActivePlayerID())
	}

	g.Unsubscribe("player1")
	g.Unsubscribe("player2")
}

func drainEvents(ch <-chan GameEvent) []GameEvent {
	var events []GameEvent
	for {
		select {
		case e := <-ch:
			events = append(events, e)
		default:
			return events
		}
	}
}
