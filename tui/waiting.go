package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/lucasacastro/qwixx/lobby"
)

// WaitingModel handles the waiting room screen.
type WaitingModel struct {
	roomCode  string
	players   []string
	isCreator bool
	started   bool
	err       string
	messages  []string
}

// NewWaitingModel creates a new waiting room screen.
func NewWaitingModel(roomCode string, players []string, isCreator bool) WaitingModel {
	return WaitingModel{
		roomCode:  roomCode,
		players:   players,
		isCreator: isCreator,
	}
}

// PlayerJoinedMsg is sent when a player joins the room.
type PlayerJoinedMsg struct {
	Players []string
	Message string
}

// PlayerLeftMsg is sent when a player leaves the room.
type PlayerLeftMsg struct {
	Players []string
	Message string
}

// GameStartedMsg is sent when the game starts.
type GameStartedMsg struct{}

func (m WaitingModel) Init() tea.Cmd {
	return nil
}

func (m WaitingModel) Update(msg tea.Msg) (WaitingModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyEnter:
			if m.isCreator && len(m.players) >= lobby.MinPlayersToStart {
				m.started = true
				return m, nil
			}
		case tea.KeyCtrlC, tea.KeyEsc:
			return m, tea.Quit
		}

	case PlayerJoinedMsg:
		m.players = msg.Players
		m.messages = append(m.messages, msg.Message)
		if len(m.messages) > 5 {
			m.messages = m.messages[1:]
		}

	case PlayerLeftMsg:
		m.players = msg.Players
		m.messages = append(m.messages, msg.Message)
		if len(m.messages) > 5 {
			m.messages = m.messages[1:]
		}
	}

	return m, nil
}



func (m WaitingModel) View() string {
	var b strings.Builder

	b.WriteString("\n")
	b.WriteString(TitleStyle.Render(" QWIXX "))
	b.WriteString("\n\n")

	// Room code (prominent)
	b.WriteString("Room Code: ")
	b.WriteString(HighlightStyle.Render(m.roomCode))
	b.WriteString("\n")
	b.WriteString(SubtitleStyle.Render("Share this code with your friends!"))
	b.WriteString("\n\n")

	// Player list
	b.WriteString(fmt.Sprintf("Players (%d/5):\n", len(m.players)))
	for i, name := range m.players {
		if i == 0 {
			b.WriteString(fmt.Sprintf("  %s %s\n", HighlightStyle.Render(name), SubtitleStyle.Render("(host)")))
		} else {
			b.WriteString(fmt.Sprintf("  %s\n", name))
		}
	}
	b.WriteString("\n")

	// Messages
	if len(m.messages) > 0 {
		for _, msg := range m.messages {
			b.WriteString(StatusStyle.Render(msg))
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}

	// Error
	if m.err != "" {
		b.WriteString(ErrorStyle.Render(m.err))
		b.WriteString("\n\n")
	}

	// Action prompt
	if m.isCreator {
		if len(m.players) < lobby.MinPlayersToStart {
			b.WriteString(SubtitleStyle.Render(fmt.Sprintf("Waiting for at least %d players to start...", lobby.MinPlayersToStart)))
		} else {
			b.WriteString(SuccessStyle.Render("Press Enter to start the game!"))
		}
	} else {
		b.WriteString(SubtitleStyle.Render("Waiting for host to start the game..."))
	}

	return b.String()
}

// Started returns true if the creator pressed start.
func (m WaitingModel) Started() bool {
	return m.started
}

// SetError sets an error message.
func (m *WaitingModel) SetError(err string) {
	m.err = err
	m.started = false
}

// UpdatePlayers updates the player list.
func (m *WaitingModel) UpdatePlayers(players []string) {
	m.players = players
}

// SetCreator updates the creator status.
func (m *WaitingModel) SetCreator(isCreator bool) {
	m.isCreator = isCreator
}
