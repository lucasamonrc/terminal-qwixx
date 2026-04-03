package lobby

import (
	"fmt"
	"math/rand"
	"sync"
	"time"

	"github.com/lucasacastro/qwixx/game"
)

const (
	MaxPlayersPerRoom = 5
	MinPlayersToStart = 2
	RoomCodeLength    = 4
	DefaultTimeout    = 30 * time.Second
)

// Lobby manages all active rooms.
type Lobby struct {
	mu    sync.RWMutex
	rooms map[string]*Room
	rng   *rand.Rand
}

// NewLobby creates a new lobby.
func NewLobby() *Lobby {
	return &Lobby{
		rooms: make(map[string]*Room),
		rng:   rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// CreateRoom creates a new room and returns its code.
func (l *Lobby) CreateRoom(creatorID, nickname string) (*Room, string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	code := l.generateCode()
	room := &Room{
		Code:        code,
		State:       RoomWaiting,
		Creator:     creatorID,
		Players:     []*Player{{ID: creatorID, Nickname: nickname}},
		subscribers: make(map[string]chan RoomEvent),
	}

	l.rooms[code] = room
	return room, code
}

// JoinRoom adds a player to an existing room.
func (l *Lobby) JoinRoom(code, playerID, nickname string) (*Room, error) {
	l.mu.RLock()
	room, ok := l.rooms[code]
	l.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("room %s not found", code)
	}

	room.mu.Lock()
	defer room.mu.Unlock()

	if room.State != RoomWaiting {
		return nil, fmt.Errorf("game already in progress")
	}

	if len(room.Players) >= MaxPlayersPerRoom {
		return nil, fmt.Errorf("room is full (%d/%d)", len(room.Players), MaxPlayersPerRoom)
	}

	// Check for duplicate player
	for _, p := range room.Players {
		if p.ID == playerID {
			return nil, fmt.Errorf("you are already in this room")
		}
		if p.Nickname == nickname {
			return nil, fmt.Errorf("nickname '%s' is already taken in this room", nickname)
		}
	}

	room.Players = append(room.Players, &Player{
		ID:       playerID,
		Nickname: nickname,
	})

	room.broadcast(RoomEvent{
		Type:    EventPlayerJoined,
		Player:  nickname,
		Message: fmt.Sprintf("%s joined the room", nickname),
	})

	return room, nil
}

// GetRoom returns a room by code.
func (l *Lobby) GetRoom(code string) *Room {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.rooms[code]
}

// RemovePlayer removes a player from their room.
// If the room is empty after removal, it's deleted.
func (l *Lobby) RemovePlayer(code, playerID string) {
	// First, look up the room under read lock
	l.mu.RLock()
	room, ok := l.rooms[code]
	l.mu.RUnlock()

	if !ok {
		return
	}

	// Operate on the room without holding lobby lock
	room.mu.Lock()

	// Remove the player
	nickname := ""
	for i, p := range room.Players {
		if p.ID == playerID {
			nickname = p.Nickname
			room.Players = append(room.Players[:i], room.Players[i+1:]...)
			break
		}
	}

	isEmpty := len(room.Players) == 0

	if nickname != "" && !isEmpty {
		room.broadcast(RoomEvent{
			Type:    EventPlayerLeft,
			Player:  nickname,
			Message: fmt.Sprintf("%s left the room", nickname),
		})
	}

	// If the creator left, assign to the next player
	if !isEmpty && room.Creator == playerID {
		room.Creator = room.Players[0].ID
		room.broadcast(RoomEvent{
			Type:    EventNewCreator,
			Player:  room.Players[0].Nickname,
			Message: fmt.Sprintf("%s is now the host", room.Players[0].Nickname),
		})
	}

	// Grab game reference before unlocking
	var g *game.Game
	if room.State == RoomPlaying && room.Game != nil {
		g = room.Game
	}

	room.mu.Unlock()

	// Unsubscribe from event channels
	room.UnsubscribeRoomEvents(playerID)

	// Notify game engine outside of room lock to avoid deadlock
	if g != nil {
		g.Unsubscribe(playerID)
		g.DisconnectPlayer(playerID)
	}

	// If room is empty, delete it under lobby write lock
	if isEmpty {
		l.mu.Lock()
		// Double-check the room is still empty (race with concurrent join)
		room.mu.RLock()
		stillEmpty := len(room.Players) == 0
		room.mu.RUnlock()
		if stillEmpty {
			delete(l.rooms, code)
		}
		l.mu.Unlock()
	}
}

// RemovePlayerByID searches all rooms for a player and removes them.
func (l *Lobby) RemovePlayerByID(playerID string) {
	l.mu.RLock()
	var foundCode string
	for code, room := range l.rooms {
		room.mu.RLock()
		for _, p := range room.Players {
			if p.ID == playerID {
				foundCode = code
				room.mu.RUnlock()
				goto found
			}
		}
		room.mu.RUnlock()
	}
found:
	l.mu.RUnlock()

	if foundCode != "" {
		l.RemovePlayer(foundCode, playerID)
	}
}

// DeleteRoom removes a room entirely.
func (l *Lobby) DeleteRoom(code string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.rooms, code)
}

// generateCode generates a unique 4-character room code.
func (l *Lobby) generateCode() string {
	const chars = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789" // Excluding I, O, 0, 1 to avoid confusion
	for attempts := 0; attempts < 1000; attempts++ {
		code := make([]byte, RoomCodeLength)
		for i := range code {
			code[i] = chars[l.rng.Intn(len(chars))]
		}
		codeStr := string(code)
		if _, exists := l.rooms[codeStr]; !exists {
			return codeStr
		}
	}
	// Fallback: extremely unlikely
	return fmt.Sprintf("%04d", l.rng.Intn(10000))
}

// RoomState represents the state of a room.
type RoomState int

const (
	RoomWaiting RoomState = iota
	RoomPlaying
	RoomFinished
)

// Room represents a game room.
type Room struct {
	mu      sync.RWMutex
	Code    string
	State   RoomState
	Creator string // Player ID of the room creator
	Players []*Player
	Game    *game.Game

	// Per-subscriber room event broadcast
	subscriberMu sync.RWMutex
	subscribers  map[string]chan RoomEvent
}

// Player represents a player in a room (lobby-level info).
type Player struct {
	ID       string
	Nickname string
}

// RoomEvent represents a lobby-level event.
type RoomEvent struct {
	Type    RoomEventType
	Player  string
	Message string
}

// RoomEventType enumerates room event types.
type RoomEventType int

const (
	EventPlayerJoined RoomEventType = iota
	EventPlayerLeft
	EventNewCreator
	EventGameStarted
	EventGameEnded
)

// SubscribeRoomEvents registers a player to receive room events.
func (r *Room) SubscribeRoomEvents(playerID string) <-chan RoomEvent {
	r.subscriberMu.Lock()
	defer r.subscriberMu.Unlock()

	ch := make(chan RoomEvent, 50)
	r.subscribers[playerID] = ch
	return ch
}

// UnsubscribeRoomEvents removes a player's room event subscription.
func (r *Room) UnsubscribeRoomEvents(playerID string) {
	r.subscriberMu.Lock()
	defer r.subscriberMu.Unlock()

	if ch, ok := r.subscribers[playerID]; ok {
		close(ch)
		delete(r.subscribers, playerID)
	}
}

// broadcast sends an event to all room subscribers.
func (r *Room) broadcast(event RoomEvent) {
	r.subscriberMu.RLock()
	defer r.subscriberMu.RUnlock()

	for _, ch := range r.subscribers {
		select {
		case ch <- event:
		default:
		}
	}
}

// StartGame starts the game in this room. Only the creator can start.
func (r *Room) StartGame(playerID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if playerID != r.Creator {
		return fmt.Errorf("only the host can start the game")
	}

	if r.State != RoomWaiting {
		return fmt.Errorf("game already started")
	}

	if len(r.Players) < MinPlayersToStart {
		return fmt.Errorf("need at least %d players to start", MinPlayersToStart)
	}

	// Create player states for the game engine
	playerStates := make([]*game.PlayerState, len(r.Players))
	for i, p := range r.Players {
		playerStates[i] = &game.PlayerState{
			ID:        p.ID,
			Nickname:  p.Nickname,
			Connected: true,
		}
	}

	r.Game = game.NewGame(playerStates, DefaultTimeout)
	r.State = RoomPlaying

	// Start the authoritative game loop (owns timer, auto-roll, auto-pass)
	r.Game.StartGameLoop()

	r.broadcast(RoomEvent{
		Type:    EventGameStarted,
		Message: "Game started!",
	})

	return nil
}

// ResetForNewGame resets the room for a new game (rematch).
func (r *Room) ResetForNewGame(playerID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if playerID != r.Creator {
		return fmt.Errorf("only the host can start a new game")
	}

	if r.State != RoomFinished {
		return fmt.Errorf("current game is not finished")
	}

	// Remove disconnected players from the room
	connected := make([]*Player, 0, len(r.Players))
	for _, p := range r.Players {
		// Check if the player is still subscribed (a proxy for connected)
		r.subscriberMu.RLock()
		_, subscribed := r.subscribers[p.ID]
		r.subscriberMu.RUnlock()
		if subscribed {
			connected = append(connected, p)
		}
	}
	r.Players = connected

	r.Game = nil
	r.State = RoomWaiting

	return nil
}

// GetPlayerNicknames returns the nicknames of all players in the room.
func (r *Room) GetPlayerNicknames() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, len(r.Players))
	for i, p := range r.Players {
		names[i] = p.Nickname
	}
	return names
}

// PlayerCount returns the number of players in the room.
func (r *Room) PlayerCount() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.Players)
}

// IsCreator returns true if the given player is the room creator.
func (r *Room) IsCreator(playerID string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.Creator == playerID
}

// MarkFinished marks the room as finished.
func (r *Room) MarkFinished() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.State = RoomFinished
}

// GetState returns the current room state.
func (r *Room) GetState() RoomState {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.State
}
