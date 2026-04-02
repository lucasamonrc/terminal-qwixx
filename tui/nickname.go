package tui

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

// NicknameModel handles the nickname entry screen.
type NicknameModel struct {
	input    textinput.Model
	err      string
	finished bool
	nickname string
}

// NewNicknameModel creates a new nickname input screen.
func NewNicknameModel() NicknameModel {
	ti := textinput.New()
	ti.Placeholder = "Enter your nickname..."
	ti.CharLimit = 15
	ti.Width = 20
	ti.Focus()

	return NicknameModel{
		input: ti,
	}
}

func (m NicknameModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m NicknameModel) Update(msg tea.Msg) (NicknameModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyEnter:
			name := strings.TrimSpace(m.input.Value())
			if len(name) < 2 {
				m.err = "Nickname must be at least 2 characters"
				return m, nil
			}
			if len(name) > 15 {
				m.err = "Nickname must be 15 characters or less"
				return m, nil
			}
			m.nickname = name
			m.finished = true
			return m, nil

		case tea.KeyCtrlC, tea.KeyEsc:
			return m, tea.Quit
		}
	}

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	m.err = ""
	return m, cmd
}

func (m NicknameModel) View() string {
	var b strings.Builder

	b.WriteString("\n")
	b.WriteString(TitleStyle.Render(" QWIXX "))
	b.WriteString("\n\n")
	b.WriteString("Welcome! What should we call you?\n\n")
	b.WriteString(m.input.View())
	b.WriteString("\n")

	if m.err != "" {
		b.WriteString("\n")
		b.WriteString(ErrorStyle.Render(m.err))
	}

	b.WriteString("\n\n")
	b.WriteString(SubtitleStyle.Render("Press Enter to continue"))

	return b.String()
}

// Nickname returns the entered nickname.
func (m NicknameModel) Nickname() string {
	return m.nickname
}

// Done returns true if the user finished entering their nickname.
func (m NicknameModel) Done() bool {
	return m.finished
}
