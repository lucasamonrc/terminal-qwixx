package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/lucasacastro/qwixx/game"
)

// BoardModel handles the main game board screen.
type BoardModel struct {
	playerID     string
	nickname     string
	gameState    *game.Game
	validMoves   []game.Move
	selectedMove int
	statusMsg    string
	messages     []string
	submitted    bool
	chosenMove   *game.Move // nil = pass
}

// NewBoardModel creates a new game board screen.
func NewBoardModel(playerID, nickname string, gameState *game.Game) BoardModel {
	return BoardModel{
		playerID:  playerID,
		nickname:  nickname,
		gameState: gameState,
	}
}

// GameStateUpdatedMsg tells the board to refresh.
type GameStateUpdatedMsg struct {
	Message string
}

// TimerTickMsg is sent every second to update the timer.
type TimerTickMsg struct{}

func (m BoardModel) Init() tea.Cmd {
	return nil
}

func (m BoardModel) Update(msg tea.Msg) (BoardModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		return m.handleKey(msg)

	case GameStateUpdatedMsg:
		if msg.Message != "" {
			m.messages = append(m.messages, msg.Message)
			if len(m.messages) > 5 {
				m.messages = m.messages[1:]
			}
		}
		m.refreshMoves()

	case TimerTickMsg:
		// Timer display is handled by reading gameState.GetTimeRemaining()
	}

	return m, nil
}

func (m BoardModel) handleKey(msg tea.KeyMsg) (BoardModel, tea.Cmd) {
	switch msg.Type {
	case tea.KeyCtrlC:
		return m, tea.Quit
	}

	if m.submitted || len(m.validMoves) == 0 {
		// Check for pass
		switch msg.String() {
		case "p", "P":
			if !m.submitted && m.canAct() {
				m.submitted = true
				m.chosenMove = nil
			}
		}
		return m, nil
	}

	switch msg.String() {
	case "p", "P":
		if m.canAct() {
			m.submitted = true
			m.chosenMove = nil
		}

	case "1", "2", "3", "4", "5", "6", "7", "8", "9":
		idx := int(msg.String()[0]-'0') - 1
		if idx < len(m.validMoves) && m.canAct() {
			m.selectedMove = idx
			m.submitted = true
			move := m.validMoves[idx]
			m.chosenMove = &move
		}

	case "up", "k":
		if m.selectedMove > 0 {
			m.selectedMove--
		}
	case "down", "j":
		if m.selectedMove < len(m.validMoves)-1 {
			m.selectedMove++
		}
	case "enter":
		if m.canAct() && m.selectedMove < len(m.validMoves) {
			m.submitted = true
			move := m.validMoves[m.selectedMove]
			m.chosenMove = &move
		}
	}

	return m, nil
}

func (m BoardModel) View() string {
	if m.gameState == nil {
		return "Loading..."
	}

	var b strings.Builder

	// Header
	b.WriteString(m.renderHeader())
	b.WriteString("\n")

	// Dice
	b.WriteString(m.renderDice())
	b.WriteString("\n\n")

	// Scorecard
	b.WriteString(m.renderScorecard())
	b.WriteString("\n")

	// Penalties
	b.WriteString(m.renderPenalties())
	b.WriteString("\n\n")

	// Players
	b.WriteString(m.renderPlayers())
	b.WriteString("\n")

	// Messages
	if len(m.messages) > 0 {
		for _, msg := range m.messages {
			b.WriteString(StatusStyle.Render(msg))
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}

	// Move selection / waiting
	b.WriteString(m.renderInput())

	return b.String()
}

func (m BoardModel) renderHeader() string {
	g := m.gameState

	activeNick := g.GetActivePlayerNickname()
	phase := g.GetPhase().String()
	timer := g.GetTimeRemaining()
	turnNum := g.GetTurnNumber()

	header := fmt.Sprintf(" Turn %d ", turnNum)
	active := fmt.Sprintf(" %s's turn ", activeNick)
	phaseStr := fmt.Sprintf(" %s ", phase)
	timerStr := fmt.Sprintf(" %ds ", timer)

	return lipgloss.JoinHorizontal(lipgloss.Center,
		TitleStyle.Render(header),
		"  ",
		HighlightStyle.Render(active),
		"  ",
		SubtitleStyle.Render(phaseStr),
		"  ",
		m.timerStyle(timer).Render(timerStr),
	)
}

func (m BoardModel) timerStyle(seconds int) lipgloss.Style {
	if seconds <= 10 {
		return ErrorStyle
	}
	return SubtitleStyle
}

func (m BoardModel) renderDice() string {
	roll := m.gameState.GetCurrentRoll()

	if roll == nil {
		return SubtitleStyle.Render("  Rolling dice...")
	}

	var dice []string

	dice = append(dice, WhiteDieStyle.Render(fmt.Sprintf("%d", roll.White1)))
	dice = append(dice, WhiteDieStyle.Render(fmt.Sprintf("%d", roll.White2)))

	if roll.ActiveColors[game.Red] {
		dice = append(dice, RedDieStyle.Render(fmt.Sprintf("%d", roll.Red)))
	}
	if roll.ActiveColors[game.Yellow] {
		dice = append(dice, YellowDieStyle.Render(fmt.Sprintf("%d", roll.Yellow)))
	}
	if roll.ActiveColors[game.Green] {
		dice = append(dice, GreenDieStyle.Render(fmt.Sprintf("%d", roll.Green)))
	}
	if roll.ActiveColors[game.Blue] {
		dice = append(dice, BlueDieStyle.Render(fmt.Sprintf("%d", roll.Blue)))
	}

	diceRow := "  " + strings.Join(dice, "  ")

	whiteSum := fmt.Sprintf("  White sum: %s", HighlightStyle.Render(fmt.Sprintf("%d", roll.WhiteSum())))

	return diceRow + "\n" + whiteSum
}

func (m BoardModel) renderScorecard() string {
	g := m.gameState
	players := g.GetPlayers()
	lockedRows := g.GetLockedRows()

	var player *game.PlayerState
	for _, p := range players {
		if p.ID == m.playerID {
			player = p
			break
		}
	}

	if player == nil {
		return "Error: player not found"
	}

	var rows []string
	for _, c := range game.AllColors {
		rows = append(rows, m.renderRow(player.Scorecard, c, lockedRows))
	}

	return strings.Join(rows, "\n")
}

func (m BoardModel) renderRow(sc *game.Scorecard, c game.Color, lockedRows map[game.Color]string) string {
	nums := game.RowNumbers(c)
	_, isLocked := lockedRows[c]

	colorStyle, markedStyle := m.getColorStyles(c)

	// Row label
	label := colorStyle.Render(fmt.Sprintf("  %-4s", c.String()[:3]))

	// Numbers
	var numStrs []string
	for _, n := range nums {
		numStr := fmt.Sprintf("%2d", n)
		if isLocked {
			numStrs = append(numStrs, LockedStyle.Render(numStr))
		} else if sc.IsMarked(c, n) {
			numStrs = append(numStrs, markedStyle.Render(numStr))
		} else {
			numStrs = append(numStrs, DimStyle.Render(numStr))
		}
	}

	// Lock indicator
	lockStr := " "
	if isLocked {
		lockStr = LockedStyle.Render("LOCKED")
	} else if sc.CanLock(c) {
		lockStr = colorStyle.Render("LOCK!")
	}

	return label + " " + strings.Join(numStrs, " ") + "  " + lockStr
}

func (m BoardModel) getColorStyles(c game.Color) (lipgloss.Style, lipgloss.Style) {
	switch c {
	case game.Red:
		return RedStyle, RedMarkedStyle
	case game.Yellow:
		return YellowStyle, YellowMarkedStyle
	case game.Green:
		return GreenStyle, GreenMarkedStyle
	case game.Blue:
		return BlueStyle, BlueMarkedStyle
	default:
		return DimStyle, DimStyle
	}
}

func (m BoardModel) renderPenalties() string {
	players := m.gameState.GetPlayers()

	var player *game.PlayerState
	for _, p := range players {
		if p.ID == m.playerID {
			player = p
			break
		}
	}

	if player == nil {
		return ""
	}

	var boxes []string
	for i := 0; i < 4; i++ {
		if i < player.Scorecard.Penalties {
			boxes = append(boxes, PenaltyBoxFilled)
		} else {
			boxes = append(boxes, PenaltyBoxEmpty)
		}
	}

	return "  Penalties: " + strings.Join(boxes, "")
}

func (m BoardModel) renderPlayers() string {
	players := m.gameState.GetPlayers()
	activeID := m.gameState.GetActivePlayerID()

	var parts []string
	for _, p := range players {
		name := p.Nickname
		if p.ID == m.playerID {
			name += "(you)"
		}
		if !p.Connected {
			name += SubtitleStyle.Render("(dc)")
		}
		penStr := ""
		if p.Scorecard.Penalties > 0 {
			penStr = ErrorStyle.Render(fmt.Sprintf("[%d]", p.Scorecard.Penalties))
		}
		if p.ID == activeID {
			parts = append(parts, HighlightStyle.Render(name)+penStr)
		} else {
			parts = append(parts, name+penStr)
		}
	}

	return "  Players: " + strings.Join(parts, " | ")
}

func (m BoardModel) renderInput() string {
	if m.submitted {
		return StatusStyle.Render("  Waiting for other players...")
	}

	if !m.canAct() {
		phase := m.gameState.GetPhase()

		if phase == game.PhaseColorCombo {
			activeNick := m.gameState.GetActivePlayerNickname()
			return SubtitleStyle.Render(fmt.Sprintf("  Waiting for %s to pick a color combo...", activeNick))
		}
		return SubtitleStyle.Render("  Waiting...")
	}

	var b strings.Builder

	if len(m.validMoves) > 0 {
		for i, mv := range m.validMoves {
			prefix := "  "
			if i == m.selectedMove {
				prefix = PromptStyle.Render("> ")
			}
			colorStyle, _ := m.getColorStyles(mv.Color)
			label := fmt.Sprintf("%d) %s %d", i+1, colorStyle.Render(mv.Color.String()), mv.Number)
			b.WriteString(prefix + label + "\n")
		}
	}

	b.WriteString("  ")
	b.WriteString(SubtitleStyle.Render("p) Pass"))
	b.WriteString("\n\n")
	b.WriteString(SubtitleStyle.Render("  Select a move or pass"))

	return b.String()
}

func (m BoardModel) canAct() bool {
	g := m.gameState
	phase := g.GetPhase()

	switch phase {
	case game.PhaseWhiteSum:
		return !g.HasPlayerActedPhase1(m.playerID)
	case game.PhaseColorCombo:
		return g.GetActivePlayerID() == m.playerID
	default:
		return false
	}
}

func (m *BoardModel) refreshMoves() {
	phase := m.gameState.GetPhase()

	switch phase {
	case game.PhaseWhiteSum:
		m.validMoves = m.gameState.GetValidMovesPhase1(m.playerID)
	case game.PhaseColorCombo:
		m.validMoves = m.gameState.GetValidMovesPhase2(m.playerID)
	default:
		m.validMoves = nil
	}
	m.selectedMove = 0
	m.submitted = false
	m.chosenMove = nil
}

// Submitted returns true if the player has submitted a move.
func (m BoardModel) Submitted() bool {
	return m.submitted
}

// ChosenMove returns the chosen move (nil for pass).
func (m BoardModel) ChosenMove() *game.Move {
	return m.chosenMove
}

// ResetSubmission resets the submission state for a new phase.
func (m *BoardModel) ResetSubmission() {
	m.submitted = false
	m.chosenMove = nil
	m.refreshMoves()
}

// SetStatusMsg sets a status message.
func (m *BoardModel) SetStatusMsg(msg string) {
	m.statusMsg = msg
}
