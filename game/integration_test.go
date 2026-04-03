package game

import (
	"testing"
	"time"
)

// TestFullGamePassOnly simulates a complete 2-player game where both players
// always pass, causing the active player to accumulate penalties until
// someone hits 4 and the game ends.
func TestFullGamePassOnly(t *testing.T) {
	players := makeTestPlayers(2)
	g := NewGame(players, 30*time.Second)

	// Subscribe both players
	ch1 := g.Subscribe("player1")
	ch2 := g.Subscribe("player2")

	// Start game loop
	g.StartGameLoop()
	defer g.StopGameLoop()

	// Wait for initial dice roll
	time.Sleep(200 * time.Millisecond)

	turnCount := 0
	maxTurns := 20 // Safety limit

	for turnCount < maxTurns {
		phase := g.GetPhase()
		if phase == PhaseGameOver {
			break
		}

		if phase == PhaseWhiteSum {
			// Both players pass Phase 1
			g.SubmitPhase1Move("player1", nil)
			g.SubmitPhase1Move("player2", nil)

			// Wait for phase transition
			time.Sleep(100 * time.Millisecond)
		}

		phase = g.GetPhase()
		if phase == PhaseColorCombo {
			// Active player passes Phase 2
			activeID := g.GetActivePlayerID()
			g.SubmitPhase2Move(activeID, nil)

			// Wait for next turn roll
			time.Sleep(1500 * time.Millisecond) // Game loop ticks every 1s

			turnCount++
		} else if phase == PhaseGameOver {
			break
		} else {
			// Wait a bit for the game loop to process
			time.Sleep(500 * time.Millisecond)
		}
	}

	if g.GetPhase() != PhaseGameOver {
		t.Fatalf("game should have ended after %d turns, phase is %v", turnCount, g.GetPhase())
	}

	// One player should have 4 penalties
	scores := g.GetScores()
	foundPenalized := false
	for _, s := range scores {
		if s.Penalties == 4 {
			foundPenalized = true
			t.Logf("%s ended with 4 penalties (score: %d)", s.Nickname, s.Total)
		} else {
			t.Logf("%s ended with %d penalties (score: %d)", s.Nickname, s.Penalties, s.Total)
		}
	}

	if !foundPenalized {
		t.Error("expected one player to have 4 penalties")
	}

	reason := g.GetGameOverReason()
	t.Logf("Game over reason: %s", reason)
	t.Logf("Game lasted %d turns", turnCount)

	// Drain event channels to verify events were delivered
	p1Events := drainEvents(ch1)
	p2Events := drainEvents(ch2)

	t.Logf("Player1 received %d events", len(p1Events))
	t.Logf("Player2 received %d events", len(p2Events))

	// Both should have received events (fan-out broadcast)
	if len(p1Events) == 0 {
		t.Error("player1 should have received events")
	}
	if len(p2Events) == 0 {
		t.Error("player2 should have received events")
	}

	// Both should have received approximately the same number of events
	diff := len(p1Events) - len(p2Events)
	if diff < 0 {
		diff = -diff
	}
	if diff > 5 {
		t.Errorf("event count difference too large: player1=%d, player2=%d", len(p1Events), len(p2Events))
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

	time.Sleep(200 * time.Millisecond)

	turnCount := 0
	maxTurns := 30

	for turnCount < maxTurns {
		phase := g.GetPhase()
		if phase == PhaseGameOver {
			break
		}

		if phase == PhaseWhiteSum {
			// Each player tries to make a move if available
			for _, id := range []string{"player1", "player2", "player3"} {
				moves := g.GetValidMovesPhase1(id)
				if len(moves) > 0 {
					g.SubmitPhase1Move(id, &moves[0])
				} else {
					g.SubmitPhase1Move(id, nil)
				}
			}
			time.Sleep(100 * time.Millisecond)
		}

		phase = g.GetPhase()
		if phase == PhaseColorCombo {
			activeID := g.GetActivePlayerID()
			moves := g.GetValidMovesPhase2(activeID)
			if len(moves) > 0 {
				g.SubmitPhase2Move(activeID, &moves[0])
			} else {
				g.SubmitPhase2Move(activeID, nil)
			}

			time.Sleep(1500 * time.Millisecond)
			turnCount++
		} else if phase == PhaseGameOver {
			break
		} else {
			time.Sleep(500 * time.Millisecond)
		}
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

// TestGameLoopAutoRollsAfterNormalTurn verifies the critical fix:
// after a normal turn completes, the game loop auto-rolls dice.
func TestGameLoopAutoRollsAfterNormalTurn(t *testing.T) {
	players := makeTestPlayers(2)
	g := NewGame(players, 30*time.Second)

	g.Subscribe("player1")
	g.Subscribe("player2")

	g.StartGameLoop()
	defer g.StopGameLoop()

	// Wait for initial roll
	time.Sleep(200 * time.Millisecond)

	if g.GetPhase() != PhaseWhiteSum {
		t.Fatalf("expected PhaseWhiteSum after start, got %v", g.GetPhase())
	}

	// Both pass Phase 1
	g.SubmitPhase1Move("player1", nil)
	g.SubmitPhase1Move("player2", nil)

	time.Sleep(100 * time.Millisecond)

	if g.GetPhase() != PhaseColorCombo {
		t.Fatalf("expected PhaseColorCombo, got %v", g.GetPhase())
	}

	// Active player passes Phase 2 (gets penalty, turn ends)
	g.SubmitPhase2Move("player1", nil)

	// The phase should now be PhaseRolling, then the game loop
	// should auto-roll within ~1 second
	time.Sleep(1500 * time.Millisecond)

	phase := g.GetPhase()
	if phase != PhaseWhiteSum {
		t.Errorf("expected PhaseWhiteSum after game loop auto-roll, got %v", phase)
	}

	// Verify it's now player2's turn
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
