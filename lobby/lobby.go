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
		Code:    code,
		State:   RoomWaiting,
		Creator: creatorID,
		Players: []*Player{
			{
				ID:       creatorID,
				Nickname: nickname,
			},
		},
		events: make(chan RoomEvent, 50),
	}

	l.rooms[code] = room
	return room, code
}

// JoinRoom adds a player to an existing room.
func (l *Lobby) JoinRoom(code, playerID, nickname string) (*Room, error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	room, ok := l.rooms[code]
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
	}

	room.Players = append(room.Players, &Player{
		ID:       playerID,
		Nickname: nickname,
	})

	room.emitEvent(RoomEvent{
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
	l.mu.Lock()
	defer l.mu.Unlock()

	room, ok := l.rooms[code]
	if !ok {
		return
	}

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

	if nickname != "" {
		room.emitEvent(RoomEvent{
			Type:    EventPlayerLeft,
			Player:  nickname,
			Message: fmt.Sprintf("%s left the room", nickname),
		})
	}

	// If room is empty, delete it
	if len(room.Players) == 0 {
		room.mu.Unlock()
		delete(l.rooms, code)
		return
	}

	// If the creator left, assign to the next player
	if room.Creator == playerID {
		room.Creator = room.Players[0].ID
		room.emitEvent(RoomEvent{
			Type:    EventNewCreator,
			Player:  room.Players[0].Nickname,
			Message: fmt.Sprintf("%s is now the host", room.Players[0].Nickname),
		})
	}

	// If game was in progress, notify the game engine
	if room.State == RoomPlaying && room.Game != nil {
		room.Game.DisconnectPlayer(playerID)
	}

	room.mu.Unlock()
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
	for {
		code := make([]byte, RoomCodeLength)
		for i := range code {
			code[i] = chars[l.rng.Intn(len(chars))]
		}
		codeStr := string(code)
		if _, exists := l.rooms[codeStr]; !exists {
			return codeStr
		}
	}
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
	events  chan RoomEvent
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

	r.emitEvent(RoomEvent{
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

// Events returns the room event channel.
func (r *Room) Events() <-chan RoomEvent {
	return r.events
}

func (r *Room) emitEvent(event RoomEvent) {
	select {
	case r.events <- event:
	default:
	}
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
