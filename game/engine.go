package game

import (
	"fmt"
	"math/rand"
	"sync"
	"time"
)

// GamePhase represents the current phase of the game.
type GamePhase int

const (
	PhaseWaiting    GamePhase = iota // Waiting for players / game not started
	PhaseRolling                     // About to roll dice
	PhaseWhiteSum                    // Phase 1: all players can use white sum
	PhaseColorCombo                  // Phase 2: active player picks white+colored combo
	PhaseGameOver                    // Game has ended
)

func (p GamePhase) String() string {
	switch p {
	case PhaseWaiting:
		return "Waiting"
	case PhaseRolling:
		return "Rolling"
	case PhaseWhiteSum:
		return "White Sum"
	case PhaseColorCombo:
		return "Color Combo"
	case PhaseGameOver:
		return "Game Over"
	default:
		return "Unknown"
	}
}

// PlayerState holds a player's game state.
type PlayerState struct {
	ID        string
	Nickname  string
	Scorecard *Scorecard
	Connected bool
}

// GameEvent represents something that happened in the game.
type GameEvent struct {
	Type    GameEventType
	Player  string // Nickname of the player involved
	Message string // Human-readable description
}

type GameEventType int

const (
	EventDiceRolled GameEventType = iota
	EventPlayerMarked
	EventPlayerPassed
	EventPlayerPenalty
	EventRowLocked
	EventGameOver
	EventPhaseChanged
	EventTimerTick
	EventTimerExpired
)

// Game is the core game state machine.
type Game struct {
	mu sync.RWMutex

	Players      []*PlayerState
	Phase        GamePhase
	ActivePlayer int // Index into Players for current active player
	CurrentRoll  *DiceRoll
	LockedRows   map[Color]string // Color -> PlayerID who locked it
	TurnNumber   int

	// Phase 1 tracking: which players have acted (marked or passed)
	Phase1Actions map[string]bool

	// Whether the active player marked anything this turn
	// (in either phase; if not, they get a penalty)
	ActiveMarkedPhase1 bool
	ActiveMarkedPhase2 bool

	// Timer
	TurnTimeout   time.Duration
	TimeRemaining int // Seconds remaining

	// RNG
	rng *rand.Rand

	// Event channel for broadcasting state changes to TUI
	Events chan GameEvent

	// Game over reason
	GameOverReason string
}

// NewGame creates a new game with the given players.
func NewGame(players []*PlayerState, timeout time.Duration) *Game {
	g := &Game{
		Players:       players,
		Phase:         PhaseRolling,
		ActivePlayer:  0,
		LockedRows:    make(map[Color]string),
		Phase1Actions: make(map[string]bool),
		TurnTimeout:   timeout,
		TimeRemaining: int(timeout.Seconds()),
		rng:           rand.New(rand.NewSource(time.Now().UnixNano())),
		Events:        make(chan GameEvent, 100),
		TurnNumber:    1,
	}

	// Initialize scorecards
	for _, p := range players {
		p.Scorecard = NewScorecard()
	}

	return g
}

// ActivePlayerState returns the current active player.
func (g *Game) ActivePlayerState() *PlayerState {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.Players[g.ActivePlayer]
}

// RollDice rolls the dice for the current turn and transitions to Phase 1.
func (g *Game) RollDice() {
	g.mu.Lock()
	defer g.mu.Unlock()

	g.CurrentRoll = NewDiceRoll(g.LockedRows)
	g.CurrentRoll.Roll(g.rng)
	g.Phase = PhaseWhiteSum
	g.Phase1Actions = make(map[string]bool)
	g.ActiveMarkedPhase1 = false
	g.ActiveMarkedPhase2 = false
	g.TimeRemaining = int(g.TurnTimeout.Seconds())

	g.emit(GameEvent{
		Type:    EventDiceRolled,
		Player:  g.Players[g.ActivePlayer].Nickname,
		Message: fmt.Sprintf("%s rolled the dice!", g.Players[g.ActivePlayer].Nickname),
	})
	g.emit(GameEvent{
		Type:    EventPhaseChanged,
		Message: "Phase 1: All players may mark the white sum",
	})
}

// GetValidMovesPhase1 returns valid moves for a player during Phase 1.
func (g *Game) GetValidMovesPhase1(playerID string) []Move {
	g.mu.RLock()
	defer g.mu.RUnlock()

	player := g.findPlayer(playerID)
	if player == nil || g.Phase != PhaseWhiteSum {
		return nil
	}
	return ValidMovesPhase1(player.Scorecard, g.CurrentRoll.WhiteSum(), g.LockedRows)
}

// GetValidMovesPhase2 returns valid moves for the active player during Phase 2.
func (g *Game) GetValidMovesPhase2(playerID string) []Move {
	g.mu.RLock()
	defer g.mu.RUnlock()

	if g.Phase != PhaseColorCombo {
		return nil
	}
	// Only active player can act in Phase 2
	if g.Players[g.ActivePlayer].ID != playerID {
		return nil
	}
	return ValidMovesPhase2(g.Players[g.ActivePlayer].Scorecard, g.CurrentRoll, g.LockedRows)
}

// SubmitPhase1Move handles a player's Phase 1 action (mark or pass).
// Pass move is nil.
func (g *Game) SubmitPhase1Move(playerID string, move *Move) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	if g.Phase != PhaseWhiteSum {
		return fmt.Errorf("not in Phase 1")
	}

	player := g.findPlayer(playerID)
	if player == nil {
		return fmt.Errorf("player not found")
	}

	if g.Phase1Actions[playerID] {
		return fmt.Errorf("already acted this phase")
	}

	if move != nil {
		// Validate the move
		if !IsValidMove(player.Scorecard, move.Color, move.Number, g.LockedRows) {
			return fmt.Errorf("invalid move")
		}

		player.Scorecard.Mark(move.Color, move.Number)

		if g.Players[g.ActivePlayer].ID == playerID {
			g.ActiveMarkedPhase1 = true
		}

		g.emit(GameEvent{
			Type:    EventPlayerMarked,
			Player:  player.Nickname,
			Message: fmt.Sprintf("%s marked %s %d", player.Nickname, move.Color, move.Number),
		})

		// Check if this triggers a row lock
		if ShouldLockRow(player.Scorecard, move.Color, move.Number) {
			g.LockedRows[move.Color] = playerID
			g.emit(GameEvent{
				Type:    EventRowLocked,
				Player:  player.Nickname,
				Message: fmt.Sprintf("%s locked the %s row!", player.Nickname, move.Color),
			})

			// Check game end: 2 rows locked
			if g.checkGameEnd() {
				return nil
			}
		}
	} else {
		g.emit(GameEvent{
			Type:    EventPlayerPassed,
			Player:  player.Nickname,
			Message: fmt.Sprintf("%s passed", player.Nickname),
		})
	}

	g.Phase1Actions[playerID] = true

	// Check if all connected players have acted
	if g.allPlayersActed() {
		g.transitionToPhase2()
	}

	return nil
}

// SubmitPhase2Move handles the active player's Phase 2 action (mark or pass).
func (g *Game) SubmitPhase2Move(playerID string, move *Move) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	if g.Phase != PhaseColorCombo {
		return fmt.Errorf("not in Phase 2")
	}

	if g.Players[g.ActivePlayer].ID != playerID {
		return fmt.Errorf("not the active player")
	}

	player := g.Players[g.ActivePlayer]

	if move != nil {
		if !IsValidMove(player.Scorecard, move.Color, move.Number, g.LockedRows) {
			return fmt.Errorf("invalid move")
		}

		player.Scorecard.Mark(move.Color, move.Number)
		g.ActiveMarkedPhase2 = true

		g.emit(GameEvent{
			Type:    EventPlayerMarked,
			Player:  player.Nickname,
			Message: fmt.Sprintf("%s marked %s %d", player.Nickname, move.Color, move.Number),
		})

		// Check row lock
		if ShouldLockRow(player.Scorecard, move.Color, move.Number) {
			g.LockedRows[move.Color] = playerID
			g.emit(GameEvent{
				Type:    EventRowLocked,
				Player:  player.Nickname,
				Message: fmt.Sprintf("%s locked the %s row!", player.Nickname, move.Color),
			})

			if g.checkGameEnd() {
				return nil
			}
		}
	}

	// If the active player didn't mark anything in either phase, penalty
	if !g.ActiveMarkedPhase1 && !g.ActiveMarkedPhase2 {
		fourPenalties := player.Scorecard.AddPenalty()
		g.emit(GameEvent{
			Type:    EventPlayerPenalty,
			Player:  player.Nickname,
			Message: fmt.Sprintf("%s takes a penalty! (%d/4)", player.Nickname, player.Scorecard.Penalties),
		})

		if fourPenalties {
			g.endGame(fmt.Sprintf("%s got 4 penalties!", player.Nickname))
			return nil
		}
	}

	g.nextTurn()
	return nil
}

// AutoPassPhase1 auto-passes all players who haven't acted in Phase 1 (timeout).
func (g *Game) AutoPassPhase1() {
	g.mu.Lock()
	defer g.mu.Unlock()

	if g.Phase != PhaseWhiteSum {
		return
	}

	for _, p := range g.Players {
		if p.Connected && !g.Phase1Actions[p.ID] {
			g.Phase1Actions[p.ID] = true
			g.emit(GameEvent{
				Type:    EventPlayerPassed,
				Player:  p.Nickname,
				Message: fmt.Sprintf("%s auto-passed (timeout)", p.Nickname),
			})
		}
	}

	g.transitionToPhase2()
}

// AutoPassPhase2 auto-passes the active player in Phase 2 (timeout).
func (g *Game) AutoPassPhase2() {
	g.mu.Lock()
	defer g.mu.Unlock()

	if g.Phase != PhaseColorCombo {
		return
	}

	player := g.Players[g.ActivePlayer]

	// Penalty if nothing marked in either phase
	if !g.ActiveMarkedPhase1 && !g.ActiveMarkedPhase2 {
		fourPenalties := player.Scorecard.AddPenalty()
		g.emit(GameEvent{
			Type:    EventPlayerPenalty,
			Player:  player.Nickname,
			Message: fmt.Sprintf("%s takes a penalty (timeout)! (%d/4)", player.Nickname, player.Scorecard.Penalties),
		})

		if fourPenalties {
			g.endGame(fmt.Sprintf("%s got 4 penalties!", player.Nickname))
			return
		}
	}

	g.nextTurn()
}

// DisconnectPlayer marks a player as disconnected.
func (g *Game) DisconnectPlayer(playerID string) {
	g.mu.Lock()
	defer g.mu.Unlock()

	for _, p := range g.Players {
		if p.ID == playerID {
			p.Connected = false
			break
		}
	}

	// If they hadn't acted in phase 1, mark them as acted
	if g.Phase == PhaseWhiteSum && !g.Phase1Actions[playerID] {
		g.Phase1Actions[playerID] = true
		if g.allPlayersActed() {
			g.transitionToPhase2()
		}
	}

	// If they're the active player in phase 2, auto-pass
	if g.Phase == PhaseColorCombo && g.Players[g.ActivePlayer].ID == playerID {
		if !g.ActiveMarkedPhase1 && !g.ActiveMarkedPhase2 {
			player := g.Players[g.ActivePlayer]
			fourPenalties := player.Scorecard.AddPenalty()
			if fourPenalties {
				g.endGame(fmt.Sprintf("%s got 4 penalties!", player.Nickname))
				return
			}
		}
		g.nextTurn()
	}

	// Check if all players disconnected
	allDisconnected := true
	for _, p := range g.Players {
		if p.Connected {
			allDisconnected = false
			break
		}
	}
	if allDisconnected {
		g.endGame("All players disconnected")
	}
}

// GetScores returns the final scores for all players.
func (g *Game) GetScores() []PlayerScore {
	g.mu.RLock()
	defer g.mu.RUnlock()

	var scores []PlayerScore
	for _, p := range g.Players {
		rowScores := make(map[Color]int)
		for _, c := range AllColors {
			rowScores[c] = p.Scorecard.RowScore(c, g.LockedRows, p.ID)
		}
		scores = append(scores, PlayerScore{
			PlayerID:  p.ID,
			Nickname:  p.Nickname,
			RowScores: rowScores,
			Penalties: p.Scorecard.Penalties,
			Total:     p.Scorecard.TotalScore(g.LockedRows, p.ID),
		})
	}
	return scores
}

// PlayerScore holds the final score breakdown for a player.
type PlayerScore struct {
	PlayerID  string
	Nickname  string
	RowScores map[Color]int
	Penalties int
	Total     int
}

// FindPlayerByID returns a player by ID (public accessor for TUI).
func (g *Game) FindPlayerByID(playerID string) *PlayerState {
	for _, p := range g.Players {
		if p.ID == playerID {
			return p
		}
	}
	return nil
}

// GetPhase returns the current game phase (thread-safe).
func (g *Game) GetPhase() GamePhase {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.Phase
}

// GetGameOverReason returns the game over reason (thread-safe).
func (g *Game) GetGameOverReason() string {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.GameOverReason
}

// GetActivePlayerNickname returns the active player's nickname (thread-safe).
func (g *Game) GetActivePlayerNickname() string {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.Players[g.ActivePlayer].Nickname
}

// GetActivePlayerID returns the active player's ID (thread-safe).
func (g *Game) GetActivePlayerID() string {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.Players[g.ActivePlayer].ID
}

// GetTurnNumber returns the current turn number (thread-safe).
func (g *Game) GetTurnNumber() int {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.TurnNumber
}

// GetTimeRemaining returns the time remaining in seconds (thread-safe).
func (g *Game) GetTimeRemaining() int {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.TimeRemaining
}

// HasPlayerActedPhase1 checks if a player has acted in Phase 1 (thread-safe).
func (g *Game) HasPlayerActedPhase1(playerID string) bool {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.Phase1Actions[playerID]
}

// GetLockedRows returns a copy of the locked rows map (thread-safe).
func (g *Game) GetLockedRows() map[Color]string {
	g.mu.RLock()
	defer g.mu.RUnlock()
	result := make(map[Color]string, len(g.LockedRows))
	for k, v := range g.LockedRows {
		result[k] = v
	}
	return result
}

// GetCurrentRoll returns the current dice roll (thread-safe).
func (g *Game) GetCurrentRoll() *DiceRoll {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.CurrentRoll
}

// GetPlayers returns the player list (thread-safe snapshot).
func (g *Game) GetPlayers() []*PlayerState {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.Players
}

// --- Internal helpers ---

func (g *Game) findPlayer(playerID string) *PlayerState {
	for _, p := range g.Players {
		if p.ID == playerID {
			return p
		}
	}
	return nil
}

func (g *Game) allPlayersActed() bool {
	for _, p := range g.Players {
		if p.Connected && !g.Phase1Actions[p.ID] {
			return false
		}
	}
	return true
}

func (g *Game) transitionToPhase2() {
	g.Phase = PhaseColorCombo
	g.TimeRemaining = int(g.TurnTimeout.Seconds())

	// Skip disconnected active player
	if !g.Players[g.ActivePlayer].Connected {
		if !g.ActiveMarkedPhase1 {
			g.Players[g.ActivePlayer].Scorecard.AddPenalty()
		}
		g.nextTurn()
		return
	}

	g.emit(GameEvent{
		Type:    EventPhaseChanged,
		Message: fmt.Sprintf("Phase 2: %s may mark a white+color combo", g.Players[g.ActivePlayer].Nickname),
	})
}

func (g *Game) nextTurn() {
	g.TurnNumber++
	g.ActivePlayer = (g.ActivePlayer + 1) % len(g.Players)

	// Skip disconnected players
	attempts := 0
	for !g.Players[g.ActivePlayer].Connected && attempts < len(g.Players) {
		g.ActivePlayer = (g.ActivePlayer + 1) % len(g.Players)
		attempts++
	}

	if attempts >= len(g.Players) {
		g.endGame("All players disconnected")
		return
	}

	g.Phase = PhaseRolling
	g.TimeRemaining = int(g.TurnTimeout.Seconds())

	g.emit(GameEvent{
		Type:    EventPhaseChanged,
		Message: fmt.Sprintf("Turn %d: %s's turn", g.TurnNumber, g.Players[g.ActivePlayer].Nickname),
	})
}

func (g *Game) checkGameEnd() bool {
	if len(g.LockedRows) >= 2 {
		g.endGame("Two rows have been locked!")
		return true
	}
	return false
}

func (g *Game) endGame(reason string) {
	g.Phase = PhaseGameOver
	g.GameOverReason = reason
	g.emit(GameEvent{
		Type:    EventGameOver,
		Message: fmt.Sprintf("Game Over! %s", reason),
	})
}

func (g *Game) emit(event GameEvent) {
	select {
	case g.Events <- event:
	default:
		// Drop event if channel is full (shouldn't happen)
	}
}

// DecrementTimer decrements the time remaining and returns the new value.
func (g *Game) DecrementTimer() int {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.TimeRemaining--
	return g.TimeRemaining
}
