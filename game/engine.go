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

	// Per-subscriber event broadcast
	subscriberMu sync.RWMutex
	subscribers  map[string]chan GameEvent // playerID -> channel

	// Game over reason
	GameOverReason string

	// Stop channel for the game loop goroutine
	stopLoop chan struct{}
	loopDone chan struct{}
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
		subscribers:   make(map[string]chan GameEvent),
		TurnNumber:    1,
		stopLoop:      make(chan struct{}),
		loopDone:      make(chan struct{}),
	}

	// Initialize scorecards
	for _, p := range players {
		p.Scorecard = NewScorecard()
	}

	return g
}

// Subscribe registers a player to receive game events. Returns a channel.
func (g *Game) Subscribe(playerID string) <-chan GameEvent {
	g.subscriberMu.Lock()
	defer g.subscriberMu.Unlock()

	ch := make(chan GameEvent, 100)
	g.subscribers[playerID] = ch
	return ch
}

// Unsubscribe removes a player's event subscription.
func (g *Game) Unsubscribe(playerID string) {
	g.subscriberMu.Lock()
	defer g.subscriberMu.Unlock()

	if ch, ok := g.subscribers[playerID]; ok {
		close(ch)
		delete(g.subscribers, playerID)
	}
}

// StartGameLoop starts the authoritative game loop goroutine.
// This goroutine owns the timer, auto-rolls dice, and handles timeouts.
// Call this once after creating the game.
func (g *Game) StartGameLoop() {
	go g.gameLoop()
}

// StopGameLoop stops the game loop goroutine.
func (g *Game) StopGameLoop() {
	select {
	case <-g.stopLoop:
		// Already stopped
	default:
		close(g.stopLoop)
	}
	<-g.loopDone
}

func (g *Game) gameLoop() {
	defer close(g.loopDone)

	// Auto-roll dice at start
	g.RollDice()

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-g.stopLoop:
			return
		case <-ticker.C:
			g.mu.Lock()
			if g.Phase == PhaseGameOver {
				g.mu.Unlock()
				return
			}

			if g.Phase == PhaseWhiteSum || g.Phase == PhaseColorCombo {
				g.TimeRemaining--
				if g.TimeRemaining <= 0 {
					// Timeout - auto-pass
					switch g.Phase {
					case PhaseWhiteSum:
						g.autoPassPhase1Locked()
					case PhaseColorCombo:
						g.autoPassPhase2Locked()
					}

					// After auto-pass, check if we need to roll
					if g.Phase == PhaseRolling {
						g.mu.Unlock()
						g.RollDice()
						continue
					}
				} else {
					g.broadcast(GameEvent{
						Type:    EventTimerTick,
						Message: fmt.Sprintf("%ds remaining", g.TimeRemaining),
					})
				}
			}
			g.mu.Unlock()
		}
	}
}

// RollDice rolls the dice for the current turn and transitions to Phase 1.
func (g *Game) RollDice() {
	g.mu.Lock()
	defer g.mu.Unlock()

	if g.Phase != PhaseRolling {
		return // Idempotent guard
	}

	g.CurrentRoll = NewDiceRoll(g.LockedRows)
	g.CurrentRoll.Roll(g.rng)
	g.Phase = PhaseWhiteSum
	g.Phase1Actions = make(map[string]bool)
	g.ActiveMarkedPhase1 = false
	g.ActiveMarkedPhase2 = false
	g.TimeRemaining = int(g.TurnTimeout.Seconds())

	g.broadcast(GameEvent{
		Type:    EventDiceRolled,
		Player:  g.Players[g.ActivePlayer].Nickname,
		Message: fmt.Sprintf("%s rolled the dice!", g.Players[g.ActivePlayer].Nickname),
	})
	g.broadcast(GameEvent{
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
		// Validate the move number matches the white sum
		if move.Number != g.CurrentRoll.WhiteSum() {
			return fmt.Errorf("Phase 1 move must use the white sum (%d)", g.CurrentRoll.WhiteSum())
		}

		// Validate the move
		if !IsValidMove(player.Scorecard, move.Color, move.Number, g.LockedRows) {
			return fmt.Errorf("invalid move")
		}

		player.Scorecard.Mark(move.Color, move.Number)

		if g.Players[g.ActivePlayer].ID == playerID {
			g.ActiveMarkedPhase1 = true
		}

		g.broadcast(GameEvent{
			Type:    EventPlayerMarked,
			Player:  player.Nickname,
			Message: fmt.Sprintf("%s marked %s %d", player.Nickname, move.Color, move.Number),
		})

		// Check if this triggers a row lock
		if ShouldLockRow(player.Scorecard, move.Color, move.Number) {
			g.lockRow(player, move.Color)
			if g.checkGameEnd() {
				return nil
			}
		}
	} else {
		g.broadcast(GameEvent{
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
		// Validate the move matches an actual dice combo
		if !g.isValidPhase2Combo(move) {
			return fmt.Errorf("move does not match any white+colored die combination")
		}

		if !IsValidMove(player.Scorecard, move.Color, move.Number, g.LockedRows) {
			return fmt.Errorf("invalid move")
		}

		player.Scorecard.Mark(move.Color, move.Number)
		g.ActiveMarkedPhase2 = true

		g.broadcast(GameEvent{
			Type:    EventPlayerMarked,
			Player:  player.Nickname,
			Message: fmt.Sprintf("%s marked %s %d", player.Nickname, move.Color, move.Number),
		})

		// Check row lock
		if ShouldLockRow(player.Scorecard, move.Color, move.Number) {
			g.lockRow(player, move.Color)
			if g.checkGameEnd() {
				return nil
			}
		}
	}

	// If the active player didn't mark anything in either phase, penalty
	if !g.ActiveMarkedPhase1 && !g.ActiveMarkedPhase2 {
		fourPenalties := player.Scorecard.AddPenalty()
		g.broadcast(GameEvent{
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

// --- Snapshot accessors (return copies, safe for concurrent reads) ---

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

// DiceRollSnapshot is an immutable snapshot of a dice roll.
type DiceRollSnapshot struct {
	White1       int
	White2       int
	Red          int
	Yellow       int
	Green        int
	Blue         int
	ActiveColors map[Color]bool
}

// WhiteSum returns the sum of the two white dice.
func (s *DiceRollSnapshot) WhiteSum() int {
	return s.White1 + s.White2
}

// GetCurrentRollSnapshot returns a copy of the current dice roll (thread-safe).
func (g *Game) GetCurrentRollSnapshot() *DiceRollSnapshot {
	g.mu.RLock()
	defer g.mu.RUnlock()
	if g.CurrentRoll == nil {
		return nil
	}
	active := make(map[Color]bool, len(g.CurrentRoll.ActiveColors))
	for k, v := range g.CurrentRoll.ActiveColors {
		active[k] = v
	}
	return &DiceRollSnapshot{
		White1:       g.CurrentRoll.White1,
		White2:       g.CurrentRoll.White2,
		Red:          g.CurrentRoll.Red,
		Yellow:       g.CurrentRoll.Yellow,
		Green:        g.CurrentRoll.Green,
		Blue:         g.CurrentRoll.Blue,
		ActiveColors: active,
	}
}

// PlayerSnapshot is an immutable snapshot of a player's state.
type PlayerSnapshot struct {
	ID        string
	Nickname  string
	Penalties int
	Connected bool
	Marks     map[Color]map[int]bool // Copy of scorecard marks
}

// GetPlayerSnapshots returns copies of all player states (thread-safe).
func (g *Game) GetPlayerSnapshots() []PlayerSnapshot {
	g.mu.RLock()
	defer g.mu.RUnlock()

	snapshots := make([]PlayerSnapshot, len(g.Players))
	for i, p := range g.Players {
		marks := make(map[Color]map[int]bool)
		for _, c := range AllColors {
			marks[c] = make(map[int]bool)
			for k, v := range p.Scorecard.Marks[c] {
				marks[c][k] = v
			}
		}
		snapshots[i] = PlayerSnapshot{
			ID:        p.ID,
			Nickname:  p.Nickname,
			Penalties: p.Scorecard.Penalties,
			Connected: p.Connected,
			Marks:     marks,
		}
	}
	return snapshots
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
			fourPenalties := g.Players[g.ActivePlayer].Scorecard.AddPenalty()
			if fourPenalties {
				g.endGame(fmt.Sprintf("%s got 4 penalties!", g.Players[g.ActivePlayer].Nickname))
				return
			}
		}
		g.nextTurn()
		return
	}

	g.broadcast(GameEvent{
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

	g.broadcast(GameEvent{
		Type:    EventPhaseChanged,
		Message: fmt.Sprintf("Turn %d: %s's turn", g.TurnNumber, g.Players[g.ActivePlayer].Nickname),
	})
}

func (g *Game) lockRow(player *PlayerState, c Color) {
	g.LockedRows[c] = player.ID

	// Update current roll to remove the locked die
	if g.CurrentRoll != nil {
		delete(g.CurrentRoll.ActiveColors, c)
	}

	g.broadcast(GameEvent{
		Type:    EventRowLocked,
		Player:  player.Nickname,
		Message: fmt.Sprintf("%s locked the %s row!", player.Nickname, c),
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
	g.broadcast(GameEvent{
		Type:    EventGameOver,
		Message: fmt.Sprintf("Game Over! %s", reason),
	})

	// Stop the game loop
	select {
	case <-g.stopLoop:
	default:
		close(g.stopLoop)
	}
}

// isValidPhase2Combo checks that the move matches an actual white+colored die combo.
func (g *Game) isValidPhase2Combo(move *Move) bool {
	combos := g.CurrentRoll.ColorCombos()
	for _, combo := range combos {
		if combo.Color == move.Color && combo.Sum == move.Number {
			return true
		}
	}
	return false
}

// broadcast sends an event to all subscribers.
func (g *Game) broadcast(event GameEvent) {
	g.subscriberMu.RLock()
	defer g.subscriberMu.RUnlock()

	for _, ch := range g.subscribers {
		select {
		case ch <- event:
		default:
			// Drop if subscriber is full (shouldn't happen)
		}
	}
}

// AutoPassPhase1 auto-passes all players who haven't acted in Phase 1 (timeout).
func (g *Game) AutoPassPhase1() {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.autoPassPhase1Locked()
}

// autoPassPhase1Locked auto-passes all players (must hold g.mu write lock).
func (g *Game) autoPassPhase1Locked() {
	if g.Phase != PhaseWhiteSum {
		return
	}

	for _, p := range g.Players {
		if p.Connected && !g.Phase1Actions[p.ID] {
			g.Phase1Actions[p.ID] = true
			g.broadcast(GameEvent{
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
	g.autoPassPhase2Locked()
}

// autoPassPhase2Locked auto-passes the active player (must hold g.mu write lock).
func (g *Game) autoPassPhase2Locked() {
	if g.Phase != PhaseColorCombo {
		return
	}

	player := g.Players[g.ActivePlayer]

	// Penalty if nothing marked in either phase
	if !g.ActiveMarkedPhase1 && !g.ActiveMarkedPhase2 {
		fourPenalties := player.Scorecard.AddPenalty()
		g.broadcast(GameEvent{
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
