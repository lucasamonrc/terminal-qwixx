package tui

import (
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/lucasacastro/qwixx/game"
	"github.com/lucasacastro/qwixx/lobby"
)

// Screen represents the current screen in the app.
type Screen int

const (
	ScreenNickname Screen = iota
	ScreenMenu
	ScreenWaiting
	ScreenBoard
	ScreenResults
)

// AppModel is the root Bubble Tea model that routes between screens.
type AppModel struct {
	screen    Screen
	playerID  string
	nickname  string
	roomCode  string
	lobby     *lobby.Lobby
	room      *lobby.Room
	isCreator bool

	// Per-player event channels
	roomEvents <-chan lobby.RoomEvent
	gameEvents <-chan game.GameEvent

	// Sub-models
	nicknameModel NicknameModel
	menuModel     MenuModel
	waitingModel  WaitingModel
	boardModel    BoardModel
	resultsModel  ResultsModel

	width  int
	height int
}

// NewAppModel creates a new root app model.
func NewAppModel(playerID string, lob *lobby.Lobby) AppModel {
	return AppModel{
		screen:        ScreenNickname,
		playerID:      playerID,
		lobby:         lob,
		nicknameModel: NewNicknameModel(),
	}
}

func (m AppModel) Init() tea.Cmd {
	return m.nicknameModel.Init()
}

// PollRoomEventsMsg triggers polling for room events.
type PollRoomEventsMsg struct{}

// PollGameEventsMsg triggers polling for game events.
type PollGameEventsMsg struct{}

func pollRoomEvents() tea.Msg {
	time.Sleep(200 * time.Millisecond)
	return PollRoomEventsMsg{}
}

func pollGameEvents() tea.Msg {
	time.Sleep(100 * time.Millisecond)
	return PollGameEventsMsg{}
}

func (m AppModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
	}

	switch m.screen {
	case ScreenNickname:
		return m.updateNickname(msg)
	case ScreenMenu:
		return m.updateMenu(msg)
	case ScreenWaiting:
		return m.updateWaiting(msg)
	case ScreenBoard:
		return m.updateBoard(msg)
	case ScreenResults:
		return m.updateResults(msg)
	}

	return m, nil
}

func (m AppModel) View() string {
	switch m.screen {
	case ScreenNickname:
		return m.nicknameModel.View()
	case ScreenMenu:
		return m.menuModel.View()
	case ScreenWaiting:
		return m.waitingModel.View()
	case ScreenBoard:
		return m.boardModel.View()
	case ScreenResults:
		return m.resultsModel.View()
	default:
		return "Unknown screen"
	}
}

// --- Screen update handlers ---

func (m AppModel) updateNickname(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	m.nicknameModel, cmd = m.nicknameModel.Update(msg)

	if m.nicknameModel.Done() {
		m.nickname = m.nicknameModel.Nickname()
		m.screen = ScreenMenu
		m.menuModel = NewMenuModel(m.nickname)
		return m, m.menuModel.Init()
	}

	return m, cmd
}

func (m AppModel) updateMenu(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	m.menuModel, cmd = m.menuModel.Update(msg)

	switch m.menuModel.Action() {
	case MenuCreate:
		room, code := m.lobby.CreateRoom(m.playerID, m.nickname)
		m.room = room
		m.roomCode = code
		m.isCreator = true
		m.roomEvents = room.SubscribeRoomEvents(m.playerID)
		m.screen = ScreenWaiting
		m.waitingModel = NewWaitingModel(code, room.GetPlayerNicknames(), true)
		return m, func() tea.Msg { return PollRoomEventsMsg{} }

	case MenuJoin:
		code := m.menuModel.RoomCode()
		room, err := m.lobby.JoinRoom(code, m.playerID, m.nickname)
		if err != nil {
			m.menuModel.SetError(err.Error())
			return m, nil
		}
		m.room = room
		m.roomCode = code
		m.isCreator = false
		m.roomEvents = room.SubscribeRoomEvents(m.playerID)
		m.screen = ScreenWaiting
		m.waitingModel = NewWaitingModel(code, room.GetPlayerNicknames(), false)
		return m, func() tea.Msg { return PollRoomEventsMsg{} }
	}

	return m, cmd
}

func (m AppModel) updateWaiting(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg.(type) {
	case PollRoomEventsMsg:
		if m.room == nil {
			return m, nil
		}

		// Drain all available room events
		for {
			select {
			case event, ok := <-m.roomEvents:
				if !ok {
					return m, nil // Channel closed
				}
				switch event.Type {
				case lobby.EventPlayerJoined, lobby.EventPlayerLeft:
					m.waitingModel.UpdatePlayers(m.room.GetPlayerNicknames())
					innerMsg := PlayerJoinedMsg{
						Players: m.room.GetPlayerNicknames(),
						Message: event.Message,
					}
					m.waitingModel, _ = m.waitingModel.Update(innerMsg)
				case lobby.EventNewCreator:
					m.isCreator = m.room.IsCreator(m.playerID)
					m.waitingModel.SetCreator(m.isCreator)
				case lobby.EventGameStarted:
					return m.transitionToBoard()
				}
			default:
				goto doneRoomEvents
			}
		}
	doneRoomEvents:

		// Fallback: check if game started (in case we missed the event)
		if m.room.GetState() == lobby.RoomPlaying && m.room.Game != nil {
			return m.transitionToBoard()
		}

		return m, func() tea.Msg { return PollRoomEventsMsg{} }
	}

	var cmd tea.Cmd
	m.waitingModel, cmd = m.waitingModel.Update(msg)

	if m.waitingModel.Started() {
		// Creator pressed start
		err := m.room.StartGame(m.playerID)
		if err != nil {
			m.waitingModel.SetError(err.Error())
			return m, func() tea.Msg { return PollRoomEventsMsg{} }
		}
		return m.transitionToBoard()
	}

	return m, cmd
}

func (m AppModel) transitionToBoard() (tea.Model, tea.Cmd) {
	if m.screen == ScreenBoard {
		// Already on board -- idempotent guard
		return m, nil
	}

	m.screen = ScreenBoard
	m.boardModel = NewBoardModel(m.playerID, m.nickname, m.room.Game)

	// Subscribe to game events (per-player channel)
	m.gameEvents = m.room.Game.Subscribe(m.playerID)

	// Don't call RollDice -- the game loop goroutine handles that
	m.boardModel.ResetSubmission()

	return m, func() tea.Msg { return PollGameEventsMsg{} }
}

func (m AppModel) updateBoard(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg.(type) {
	case PollGameEventsMsg:
		if m.room == nil || m.room.Game == nil {
			return m, nil
		}

		g := m.room.Game
		needsRefresh := false

		// Drain all available game events
		for {
			select {
			case event, ok := <-m.gameEvents:
				if !ok {
					return m, nil // Channel closed
				}
				if event.Message != "" {
					m.boardModel.AddMessage(event.Message)
				}
				needsRefresh = true

				if event.Type == game.EventGameOver {
					return m.transitionToResults()
				}
			default:
				goto doneGameEvents
			}
		}
	doneGameEvents:

		if needsRefresh {
			m.boardModel.RefreshFromGame()
		}

		// Fallback: check game over
		if g.GetPhase() == game.PhaseGameOver {
			return m.transitionToResults()
		}

		return m, func() tea.Msg { return PollGameEventsMsg{} }
	}

	var cmd tea.Cmd
	m.boardModel, cmd = m.boardModel.Update(msg)

	// Check if the player submitted a move
	if m.boardModel.Submitted() && m.room != nil && m.room.Game != nil {
		g := m.room.Game

		switch g.GetPhase() {
		case game.PhaseWhiteSum:
			err := g.SubmitPhase1Move(m.playerID, m.boardModel.ChosenMove())
			if err != nil {
				m.boardModel.SetStatusMsg(fmt.Sprintf("Error: %v", err))
				m.boardModel.ResetSubmission()
			} else {
				// Move accepted; the game engine handles phase transitions and dice rolling
				m.boardModel.RefreshFromGame()
			}

		case game.PhaseColorCombo:
			err := g.SubmitPhase2Move(m.playerID, m.boardModel.ChosenMove())
			if err != nil {
				m.boardModel.SetStatusMsg(fmt.Sprintf("Error: %v", err))
				m.boardModel.ResetSubmission()
			} else {
				// Check if game ended
				if g.GetPhase() == game.PhaseGameOver {
					return m.transitionToResults()
				}
				m.boardModel.RefreshFromGame()
			}
		}
	}

	return m, cmd
}

func (m AppModel) transitionToResults() (tea.Model, tea.Cmd) {
	if m.screen == ScreenResults {
		return m, nil // Already on results
	}

	m.room.MarkFinished()
	scores := m.room.Game.GetScores()
	reason := m.room.Game.GetGameOverReason()

	m.screen = ScreenResults
	m.resultsModel = NewResultsModel(scores, reason, m.isCreator, m.playerID)
	return m, nil
}

func (m AppModel) updateResults(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	m.resultsModel, cmd = m.resultsModel.Update(msg)

	if m.resultsModel.PlayAgain() {
		err := m.room.ResetForNewGame(m.playerID)
		if err != nil {
			return m, cmd
		}
		m.screen = ScreenWaiting
		m.waitingModel = NewWaitingModel(m.roomCode, m.room.GetPlayerNicknames(), m.isCreator)
		return m, func() tea.Msg { return PollRoomEventsMsg{} }
	}

	return m, cmd
}

// PlayerID returns the player ID for cleanup.
func (m AppModel) PlayerID() string {
	return m.playerID
}

// RoomCode returns the current room code for cleanup.
func (m AppModel) RoomCode() string {
	return m.roomCode
}

// Cleanup unsubscribes from events and removes from room.
func (m AppModel) Cleanup() {
	if m.room != nil {
		m.room.UnsubscribeRoomEvents(m.playerID)
		if m.room.Game != nil {
			m.room.Game.Unsubscribe(m.playerID)
		}
	}
	if m.roomCode != "" {
		m.lobby.RemovePlayer(m.roomCode, m.playerID)
	}
}
