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
	PhaseWaiting  GamePhase = iota // Waiting for players / game not started
	PhaseRolling                   // About to roll dice
	PhaseAction                    // Everyone acts simultaneously on the roll
	PhaseGameOver                  // Game has ended
)

func (p GamePhase) String() string {
	switch p {
	case PhaseWaiting:
		return "Waiting"
	case PhaseRolling:
		return "Rolling"
	case PhaseAction:
		return "Action"
	case PhaseGameOver:
		return "Game Over"
	default:
		return "Unknown"
	}
}

// PlayerAction tracks what a player has done this turn.
type PlayerAction struct {
	WhiteMarked bool  // Did they mark the white sum?
	WhiteMove   *Move // The white sum move they made (nil if passed/skipped)
	ColorMarked bool  // Did they mark a color combo? (active player only)
	ColorMove   *Move // The color combo move (nil if passed/skipped)
	Confirmed   bool  // Have they finished their turn?
}

// PlayerStep indicates what a player should do next.
type PlayerStep int

const (
	StepWhite     PlayerStep = iota // Choose to mark white sum or skip
	StepColor                       // Active player: choose color combo or skip
	StepDone                        // Player is done for this turn
	StepWaiting                     // Waiting for others (already confirmed)
)

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
	EventPlayerConfirmed
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

	// Per-player action tracking for the current turn
	Actions map[string]*PlayerAction

	// RNG
	rng *rand.Rand

	// Per-subscriber event broadcast
	subscriberMu sync.RWMutex
	subscribers  map[string]chan GameEvent

	// Game over reason
	GameOverReason string

	// Stop channel for the game loop goroutine
	stopLoop chan struct{}
	loopDone chan struct{}
}

// NewGame creates a new game with the given players.
func NewGame(players []*PlayerState, timeout time.Duration) *Game {
	g := &Game{
		Players:      players,
		Phase:        PhaseRolling,
		ActivePlayer: 0,
		LockedRows:   make(map[Color]string),
		Actions:      make(map[string]*PlayerAction),
		rng:          rand.New(rand.NewSource(time.Now().UnixNano())),
		subscribers:  make(map[string]chan GameEvent),
		TurnNumber:   1,
		stopLoop:     make(chan struct{}),
		loopDone:     make(chan struct{}),
	}

	for _, p := range players {
		p.Scorecard = NewScorecard()
	}

	return g
}

// Subscribe registers a player to receive game events.
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

// StartGameLoop starts the game loop goroutine.
func (g *Game) StartGameLoop() {
	go g.gameLoop()
}

// StopGameLoop stops the game loop goroutine.
func (g *Game) StopGameLoop() {
	select {
	case <-g.stopLoop:
	default:
		close(g.stopLoop)
	}
	<-g.loopDone
}

func (g *Game) gameLoop() {
	defer close(g.loopDone)

	// Auto-roll at start
	g.RollDice()

	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-g.stopLoop:
			return
		case <-ticker.C:
			g.mu.RLock()
			phase := g.Phase
			g.mu.RUnlock()

			if phase == PhaseGameOver {
				return
			}
			if phase == PhaseRolling {
				g.RollDice()
			}
		}
	}
}

// RollDice rolls the dice and transitions to the action phase.
func (g *Game) RollDice() {
	g.mu.Lock()
	defer g.mu.Unlock()

	if g.Phase != PhaseRolling {
		return
	}

	g.CurrentRoll = NewDiceRoll(g.LockedRows)
	g.CurrentRoll.Roll(g.rng)
	g.Phase = PhaseAction

	// Initialize per-player actions
	g.Actions = make(map[string]*PlayerAction)
	for _, p := range g.Players {
		g.Actions[p.ID] = &PlayerAction{}
		// Non-active disconnected players auto-confirm
		if !p.Connected {
			g.Actions[p.ID].Confirmed = true
		}
	}

	g.broadcast(GameEvent{
		Type:    EventDiceRolled,
		Player:  g.Players[g.ActivePlayer].Nickname,
		Message: fmt.Sprintf("%s rolled the dice!", g.Players[g.ActivePlayer].Nickname),
	})
}

// GetPlayerStep returns what step a player is currently on.
func (g *Game) GetPlayerStep(playerID string) PlayerStep {
	g.mu.RLock()
	defer g.mu.RUnlock()

	if g.Phase != PhaseAction {
		return StepWaiting
	}

	action := g.Actions[playerID]
	if action == nil {
		return StepWaiting
	}

	if action.Confirmed {
		return StepWaiting
	}

	isActive := g.Players[g.ActivePlayer].ID == playerID

	// White step: hasn't decided on white sum yet
	if !action.WhiteMarked {
		return StepWhite
	}

	// Active player: after white, can do color
	if isActive && !action.ColorMarked {
		return StepColor
	}

	// Should be confirmed by now, but just in case
	return StepDone
}

// GetValidMoves returns the valid moves for a player's current step.
func (g *Game) GetValidMoves(playerID string) []Move {
	g.mu.RLock()
	defer g.mu.RUnlock()

	if g.Phase != PhaseAction {
		return nil
	}

	action := g.Actions[playerID]
	if action == nil || action.Confirmed {
		return nil
	}

	player := g.findPlayer(playerID)
	if player == nil {
		return nil
	}

	isActive := g.Players[g.ActivePlayer].ID == playerID

	if !action.WhiteMarked {
		// White sum moves
		return ValidMovesPhase1(player.Scorecard, g.CurrentRoll.WhiteSum(), g.LockedRows)
	}

	if isActive && !action.ColorMarked {
		// Color combo moves
		return ValidMovesPhase2(player.Scorecard, g.CurrentRoll, g.LockedRows)
	}

	return nil
}

// SubmitMark marks a number for a player. The engine determines which step
// this applies to based on the player's current state.
func (g *Game) SubmitMark(playerID string, move Move) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	if g.Phase != PhaseAction {
		return fmt.Errorf("not in action phase")
	}

	action := g.Actions[playerID]
	if action == nil || action.Confirmed {
		return fmt.Errorf("already confirmed")
	}

	player := g.findPlayer(playerID)
	if player == nil {
		return fmt.Errorf("player not found")
	}

	isActive := g.Players[g.ActivePlayer].ID == playerID

	if !action.WhiteMarked {
		// White sum step
		if move.Number != g.CurrentRoll.WhiteSum() {
			return fmt.Errorf("must use the white sum (%d)", g.CurrentRoll.WhiteSum())
		}
		if !IsValidMove(player.Scorecard, move.Color, move.Number, g.LockedRows) {
			return fmt.Errorf("invalid move")
		}

		player.Scorecard.Mark(move.Color, move.Number)
		action.WhiteMarked = true
		action.WhiteMove = &move

		g.broadcast(GameEvent{
			Type:    EventPlayerMarked,
			Player:  player.Nickname,
			Message: fmt.Sprintf("%s marked %s %d", player.Nickname, move.Color, move.Number),
		})

		if ShouldLockRow(player.Scorecard, move.Color, move.Number) {
			g.lockRow(player, move.Color)
			if g.checkGameEnd() {
				return nil
			}
		}

		// Non-active players auto-confirm after white step
		if !isActive {
			action.Confirmed = true
			g.checkAllConfirmed()
		}

		return nil
	}

	if isActive && !action.ColorMarked {
		// Color combo step
		if !g.isValidPhase2Combo(&move) {
			return fmt.Errorf("does not match any white+colored die combination")
		}
		if !IsValidMove(player.Scorecard, move.Color, move.Number, g.LockedRows) {
			return fmt.Errorf("invalid move")
		}

		player.Scorecard.Mark(move.Color, move.Number)
		action.ColorMarked = true
		action.ColorMove = &move

		g.broadcast(GameEvent{
			Type:    EventPlayerMarked,
			Player:  player.Nickname,
			Message: fmt.Sprintf("%s marked %s %d", player.Nickname, move.Color, move.Number),
		})

		if ShouldLockRow(player.Scorecard, move.Color, move.Number) {
			g.lockRow(player, move.Color)
			if g.checkGameEnd() {
				return nil
			}
		}

		// Active player done after color step
		action.Confirmed = true
		g.checkAllConfirmed()

		return nil
	}

	return fmt.Errorf("no pending action")
}

// SubmitPass skips the current step for a player.
func (g *Game) SubmitPass(playerID string) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	if g.Phase != PhaseAction {
		return fmt.Errorf("not in action phase")
	}

	action := g.Actions[playerID]
	if action == nil || action.Confirmed {
		return fmt.Errorf("already confirmed")
	}

	player := g.findPlayer(playerID)
	if player == nil {
		return fmt.Errorf("player not found")
	}

	isActive := g.Players[g.ActivePlayer].ID == playerID

	if !action.WhiteMarked {
		// Pass on white sum
		action.WhiteMarked = true // Marked as "decided" (but no move)

		g.broadcast(GameEvent{
			Type:    EventPlayerPassed,
			Player:  player.Nickname,
			Message: fmt.Sprintf("%s passed", player.Nickname),
		})

		// Non-active players are done
		if !isActive {
			action.Confirmed = true
			g.checkAllConfirmed()
			return nil
		}

		// Active player moves to color step (don't auto-confirm)
		return nil
	}

	if isActive && !action.ColorMarked {
		// Pass on color combo
		action.ColorMarked = true

		g.broadcast(GameEvent{
			Type:    EventPlayerPassed,
			Player:  player.Nickname,
			Message: fmt.Sprintf("%s passed on color combo", player.Nickname),
		})

		// Active player done
		action.Confirmed = true
		g.checkAllConfirmed()
		return nil
	}

	return fmt.Errorf("no pending action")
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

	// Auto-confirm if in action phase
	if g.Phase == PhaseAction {
		if action := g.Actions[playerID]; action != nil && !action.Confirmed {
			action.WhiteMarked = true
			action.ColorMarked = true
			action.Confirmed = true
			g.checkAllConfirmed()
		}
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

// --- Snapshot accessors ---

func (g *Game) GetPhase() GamePhase {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.Phase
}

func (g *Game) GetGameOverReason() string {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.GameOverReason
}

func (g *Game) GetActivePlayerNickname() string {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.Players[g.ActivePlayer].Nickname
}

func (g *Game) GetActivePlayerID() string {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.Players[g.ActivePlayer].ID
}

func (g *Game) GetTurnNumber() int {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.TurnNumber
}

func (g *Game) IsActivePlayer(playerID string) bool {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.Players[g.ActivePlayer].ID == playerID
}

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

func (s *DiceRollSnapshot) WhiteSum() int {
	return s.White1 + s.White2
}

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
	Marks     map[Color]map[int]bool
}

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

func (g *Game) checkAllConfirmed() {
	for _, p := range g.Players {
		action := g.Actions[p.ID]
		if action == nil || !action.Confirmed {
			return
		}
	}
	// Everyone confirmed -- end the turn
	g.endTurn()
}

func (g *Game) endTurn() {
	// Check if active player marked anything
	activeID := g.Players[g.ActivePlayer].ID
	action := g.Actions[activeID]

	activeMarkedAnything := (action.WhiteMove != nil) || (action.ColorMove != nil)

	if !activeMarkedAnything {
		player := g.Players[g.ActivePlayer]
		fourPenalties := player.Scorecard.AddPenalty()
		g.broadcast(GameEvent{
			Type:    EventPlayerPenalty,
			Player:  player.Nickname,
			Message: fmt.Sprintf("%s takes a penalty! (%d/4)", player.Nickname, player.Scorecard.Penalties),
		})
		if fourPenalties {
			g.endGame(fmt.Sprintf("%s got 4 penalties!", player.Nickname))
			return
		}
	}

	g.nextTurn()
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

	g.broadcast(GameEvent{
		Type:    EventPhaseChanged,
		Message: fmt.Sprintf("Turn %d: %s rolls!", g.TurnNumber, g.Players[g.ActivePlayer].Nickname),
	})
}

func (g *Game) lockRow(player *PlayerState, c Color) {
	g.LockedRows[c] = player.ID
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
	select {
	case <-g.stopLoop:
	default:
		close(g.stopLoop)
	}
}

func (g *Game) isValidPhase2Combo(move *Move) bool {
	combos := g.CurrentRoll.ColorCombos()
	for _, combo := range combos {
		if combo.Color == move.Color && combo.Sum == move.Number {
			return true
		}
	}
	return false
}

func (g *Game) broadcast(event GameEvent) {
	g.subscriberMu.RLock()
	defer g.subscriberMu.RUnlock()
	for _, ch := range g.subscribers {
		select {
		case ch <- event:
		default:
		}
	}
}
