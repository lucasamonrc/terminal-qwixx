package tui

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

// MenuAction represents the user's choice in the menu.
type MenuAction int

const (
	MenuNone MenuAction = iota
	MenuCreate
	MenuJoin
)

// MenuModel handles the create/join room menu.
type MenuModel struct {
	selected   int // 0 = Create, 1 = Join
	action     MenuAction
	codeInput  textinput.Model
	enteringCode bool
	err        string
	roomCode   string
	nickname   string
}

// NewMenuModel creates a new menu screen.
func NewMenuModel(nickname string) MenuModel {
	ti := textinput.New()
	ti.Placeholder = "XXXX"
	ti.CharLimit = 4
	ti.Width = 6

	return MenuModel{
		codeInput: ti,
		nickname:  nickname,
	}
}

func (m MenuModel) Init() tea.Cmd {
	return nil
}

func (m MenuModel) Update(msg tea.Msg) (MenuModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if m.enteringCode {
			return m.updateCodeInput(msg)
		}
		return m.updateMenuSelection(msg)
	}

	if m.enteringCode {
		var cmd tea.Cmd
		m.codeInput, cmd = m.codeInput.Update(msg)
		return m, cmd
	}

	return m, nil
}

func (m MenuModel) updateMenuSelection(msg tea.KeyMsg) (MenuModel, tea.Cmd) {
	switch msg.Type {
	case tea.KeyUp, tea.KeyShiftTab:
		m.selected = 0
	case tea.KeyDown, tea.KeyTab:
		m.selected = 1
	case tea.KeyEnter:
		if m.selected == 0 {
			m.action = MenuCreate
		} else {
			m.enteringCode = true
			m.codeInput.Focus()
			return m, textinput.Blink
		}
	case tea.KeyCtrlC, tea.KeyEsc:
		return m, tea.Quit
	default:
		switch msg.String() {
		case "c", "C":
			m.action = MenuCreate
		case "j", "J":
			m.enteringCode = true
			m.codeInput.Focus()
			return m, textinput.Blink
		}
	}
	return m, nil
}

func (m MenuModel) updateCodeInput(msg tea.KeyMsg) (MenuModel, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEnter:
		code := strings.TrimSpace(strings.ToUpper(m.codeInput.Value()))
		if len(code) != 4 {
			m.err = "Room code must be 4 characters"
			return m, nil
		}
		m.roomCode = code
		m.action = MenuJoin
		return m, nil

	case tea.KeyEsc:
		m.enteringCode = false
		m.err = ""
		m.codeInput.SetValue("")
		return m, nil

	case tea.KeyCtrlC:
		return m, tea.Quit
	}

	var cmd tea.Cmd
	m.codeInput, cmd = m.codeInput.Update(msg)
	m.err = ""
	return m, cmd
}

func (m MenuModel) View() string {
	var b strings.Builder

	b.WriteString("\n")
	b.WriteString(TitleStyle.Render(" QWIXX "))
	b.WriteString("\n\n")
	b.WriteString("Hello, ")
	b.WriteString(HighlightStyle.Render(m.nickname))
	b.WriteString("!\n\n")

	if m.enteringCode {
		b.WriteString("Enter room code:\n\n")
		b.WriteString("  ")
		b.WriteString(m.codeInput.View())
		b.WriteString("\n")

		if m.err != "" {
			b.WriteString("\n")
			b.WriteString(ErrorStyle.Render(m.err))
		}

		b.WriteString("\n\n")
		b.WriteString(SubtitleStyle.Render("Press Enter to join • Esc to go back"))
	} else {
		createPrefix := "  "
		joinPrefix := "  "
		if m.selected == 0 {
			createPrefix = PromptStyle.Render("> ")
		} else {
			joinPrefix = PromptStyle.Render("> ")
		}

		b.WriteString(createPrefix)
		if m.selected == 0 {
			b.WriteString(HighlightStyle.Render("[C]reate a room"))
		} else {
			b.WriteString("[C]reate a room")
		}
		b.WriteString("\n")

		b.WriteString(joinPrefix)
		if m.selected == 1 {
			b.WriteString(HighlightStyle.Render("[J]oin a room"))
		} else {
			b.WriteString("[J]oin a room")
		}
		b.WriteString("\n\n")

		b.WriteString(SubtitleStyle.Render("Use arrow keys or press C/J"))
	}

	return b.String()
}

// Action returns the selected menu action.
func (m MenuModel) Action() MenuAction {
	return m.action
}

// RoomCode returns the entered room code (for join action).
func (m MenuModel) RoomCode() string {
	return m.roomCode
}

// SetError sets an error message (e.g., from a failed join).
func (m *MenuModel) SetError(err string) {
	m.err = err
	m.action = MenuNone
	if m.enteringCode {
		m.codeInput.SetValue("")
	}
}
