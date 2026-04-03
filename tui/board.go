package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/lucasacastro/qwixx/game"
)

// Cursor position on the scorecard.
type cursor struct {
	row int // 0=Red, 1=Yellow, 2=Green, 3=Blue
	col int // 0-10 (index into the 11 numbers per row)
}

// BoardModel handles the main game board screen.
type BoardModel struct {
	playerID   string
	nickname   string
	gameState  *game.Game
	validMoves map[game.Color]map[int]bool // quick lookup: color -> number -> valid
	cursor     cursor
	statusMsg  string
	messages   []string
	submitted  bool
	chosenMove *game.Move // nil = pass
}

// NewBoardModel creates a new game board screen.
func NewBoardModel(playerID, nickname string, gameState *game.Game) BoardModel {
	return BoardModel{
		playerID:   playerID,
		nickname:   nickname,
		gameState:  gameState,
		validMoves: make(map[game.Color]map[int]bool),
	}
}

func (m BoardModel) Init() tea.Cmd {
	return nil
}

func (m BoardModel) Update(msg tea.Msg) (BoardModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		return m.handleKey(msg)
	}
	return m, nil
}

func (m BoardModel) handleKey(msg tea.KeyMsg) (BoardModel, tea.Cmd) {
	switch msg.Type {
	case tea.KeyCtrlC:
		return m, tea.Quit
	}

	m.statusMsg = ""

	if m.submitted || !m.canAct() {
		return m, nil
	}

	colors := game.AllColors
	rows := make([][]int, 4)
	for i, c := range colors {
		rows[i] = game.RowNumbers(c)
	}

	switch msg.String() {
	case "up", "k":
		if m.cursor.row > 0 {
			m.cursor.row--
		}
	case "down", "j":
		if m.cursor.row < 3 {
			m.cursor.row++
		}
	case "left", "h":
		if m.cursor.col > 0 {
			m.cursor.col--
		}
	case "right", "l":
		if m.cursor.col < len(rows[m.cursor.row])-1 {
			m.cursor.col++
		}
	case "enter", " ":
		// Try to select the number under the cursor
		c := colors[m.cursor.row]
		num := rows[m.cursor.row][m.cursor.col]
		if m.validMoves[c] != nil && m.validMoves[c][num] {
			m.submitted = true
			m.chosenMove = &game.Move{Color: c, Number: num}
		}
	case "p", "P":
		m.submitted = true
		m.chosenMove = nil
	}

	return m, nil
}

func (m BoardModel) View() string {
	if m.gameState == nil {
		return "Loading..."
	}

	var b strings.Builder

	b.WriteString(m.renderHeader())
	b.WriteString("\n\n")
	b.WriteString(m.renderDice())
	b.WriteString("\n\n")
	b.WriteString(m.renderScorecard())
	b.WriteString("\n")
	b.WriteString(m.renderPenalties())
	b.WriteString("\n\n")
	b.WriteString(m.renderPlayers())
	b.WriteString("\n\n")

	if m.statusMsg != "" {
		b.WriteString(ErrorStyle.Render("  " + m.statusMsg))
		b.WriteString("\n")
	}

	if len(m.messages) > 0 {
		for _, msg := range m.messages[max(0, len(m.messages)-3):] {
			b.WriteString(StatusStyle.Render("  " + msg))
			b.WriteString("\n")
		}
	}

	b.WriteString("\n")
	b.WriteString(m.renderControls())

	return b.String()
}

func (m BoardModel) renderHeader() string {
	g := m.gameState
	activeNick := g.GetActivePlayerNickname()
	phase := g.GetPhase()
	turnNum := g.GetTurnNumber()

	var phaseDesc string
	switch phase {
	case game.PhaseWhiteSum:
		phaseDesc = "Everyone: mark the white sum?"
	case game.PhaseColorCombo:
		phaseDesc = fmt.Sprintf("%s: mark a color combo?", activeNick)
	case game.PhaseRolling:
		phaseDesc = "Rolling dice..."
	default:
		phaseDesc = phase.String()
	}

	return lipgloss.JoinHorizontal(lipgloss.Center,
		TitleStyle.Render(fmt.Sprintf(" Turn %d ", turnNum)),
		"  ",
		HighlightStyle.Render(fmt.Sprintf(" %s's turn ", activeNick)),
		"  ",
		SubtitleStyle.Render(phaseDesc),
	)
}

func (m BoardModel) renderDice() string {
	roll := m.gameState.GetCurrentRollSnapshot()
	if roll == nil {
		return SubtitleStyle.Render("  Rolling dice...")
	}

	var dice []string
	dice = append(dice, WhiteDieStyle.Render(fmt.Sprintf(" %d ", roll.White1)))
	dice = append(dice, WhiteDieStyle.Render(fmt.Sprintf(" %d ", roll.White2)))

	if roll.ActiveColors[game.Red] {
		dice = append(dice, RedDieStyle.Render(fmt.Sprintf(" %d ", roll.Red)))
	}
	if roll.ActiveColors[game.Yellow] {
		dice = append(dice, YellowDieStyle.Render(fmt.Sprintf(" %d ", roll.Yellow)))
	}
	if roll.ActiveColors[game.Green] {
		dice = append(dice, GreenDieStyle.Render(fmt.Sprintf(" %d ", roll.Green)))
	}
	if roll.ActiveColors[game.Blue] {
		dice = append(dice, BlueDieStyle.Render(fmt.Sprintf(" %d ", roll.Blue)))
	}

	diceRow := "  Dice:  " + strings.Join(dice, "  ")
	whiteSum := fmt.Sprintf("      White sum = %s",
		lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FFFFFF")).Background(lipgloss.Color("#555555")).Padding(0, 1).Render(fmt.Sprintf(" %d ", roll.WhiteSum())))

	return diceRow + whiteSum
}

func (m BoardModel) renderScorecard() string {
	snapshots := m.gameState.GetPlayerSnapshots()
	lockedRows := m.gameState.GetLockedRows()

	var player *game.PlayerSnapshot
	for i := range snapshots {
		if snapshots[i].ID == m.playerID {
			player = &snapshots[i]
			break
		}
	}
	if player == nil {
		return "Error: player not found"
	}

	canAct := m.canAct()

	var rows []string
	for rowIdx, c := range game.AllColors {
		rows = append(rows, m.renderRow(player, c, lockedRows, rowIdx, canAct))
	}

	return strings.Join(rows, "\n")
}

func (m BoardModel) renderRow(player *game.PlayerSnapshot, c game.Color, lockedRows map[game.Color]string, rowIdx int, canAct bool) string {
	nums := game.RowNumbers(c)
	_, isLocked := lockedRows[c]
	colorStyle, markedStyle := m.getColorStyles(c)

	// Row label
	label := colorStyle.Render(fmt.Sprintf("  %-6s", c.String()))

	// Selectable style for valid moves under cursor
	cursorStyle := lipgloss.NewStyle().
		Background(lipgloss.Color("#FFFFFF")).
		Foreground(lipgloss.Color("#000000")).
		Bold(true)

	validStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FFFFFF")).
		Bold(true).
		Underline(true)

	// Numbers
	var numStrs []string
	for colIdx, n := range nums {
		numStr := fmt.Sprintf("%2d", n)
		isValid := m.validMoves[c] != nil && m.validMoves[c][n]
		isCursor := canAct && m.cursor.row == rowIdx && m.cursor.col == colIdx

		if isLocked {
			numStrs = append(numStrs, LockedStyle.Render(numStr))
		} else if player.Marks[c][n] {
			numStrs = append(numStrs, markedStyle.Render(numStr))
		} else if isCursor && isValid {
			numStrs = append(numStrs, cursorStyle.Render(numStr))
		} else if isCursor {
			// Cursor on non-valid number
			numStrs = append(numStrs, lipgloss.NewStyle().Foreground(lipgloss.Color("#888888")).Reverse(true).Render(numStr))
		} else if isValid {
			numStrs = append(numStrs, validStyle.Render(numStr))
		} else {
			numStrs = append(numStrs, DimStyle.Render(numStr))
		}
	}

	// Lock indicator
	lockStr := ""
	if isLocked {
		lockStr = "  " + LockedStyle.Render("LOCKED")
	}

	// Mark count
	markCount := 0
	for _, n := range nums {
		if player.Marks[c][n] {
			markCount++
		}
	}
	countStr := SubtitleStyle.Render(fmt.Sprintf(" (%d)", markCount))

	return label + strings.Join(numStrs, " ") + countStr + lockStr
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
	snapshots := m.gameState.GetPlayerSnapshots()

	var player *game.PlayerSnapshot
	for i := range snapshots {
		if snapshots[i].ID == m.playerID {
			player = &snapshots[i]
			break
		}
	}
	if player == nil {
		return ""
	}

	var boxes []string
	for i := 0; i < 4; i++ {
		if i < player.Penalties {
			boxes = append(boxes, PenaltyBoxFilled)
		} else {
			boxes = append(boxes, PenaltyBoxEmpty)
		}
	}

	return "  Penalties: " + strings.Join(boxes, "") + SubtitleStyle.Render(fmt.Sprintf("  (-%d pts)", player.Penalties*5))
}

func (m BoardModel) renderPlayers() string {
	snapshots := m.gameState.GetPlayerSnapshots()
	activeID := m.gameState.GetActivePlayerID()

	var parts []string
	for _, p := range snapshots {
		name := p.Nickname
		if p.ID == m.playerID {
			name += " (you)"
		}
		if !p.Connected {
			name += SubtitleStyle.Render(" (dc)")
		}
		penStr := ""
		if p.Penalties > 0 {
			penStr = ErrorStyle.Render(fmt.Sprintf(" [%d]", p.Penalties))
		}
		if p.ID == activeID {
			parts = append(parts, HighlightStyle.Render(name)+penStr)
		} else {
			parts = append(parts, SubtitleStyle.Render(name)+penStr)
		}
	}

	return "  Players: " + strings.Join(parts, "  |  ")
}

func (m BoardModel) renderControls() string {
	if m.submitted {
		return StatusStyle.Render("  Waiting for other players...")
	}

	if !m.canAct() {
		phase := m.gameState.GetPhase()
		if phase == game.PhaseColorCombo {
			activeNick := m.gameState.GetActivePlayerNickname()
			return SubtitleStyle.Render(fmt.Sprintf("  Waiting for %s...", activeNick))
		}
		if phase == game.PhaseRolling {
			return SubtitleStyle.Render("  Waiting for dice roll...")
		}
		return SubtitleStyle.Render("  Waiting...")
	}

	return SubtitleStyle.Render("  Arrow keys to move  |  Enter to mark  |  P to pass")
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
	m.validMoves = make(map[game.Color]map[int]bool)

	var moves []game.Move
	switch phase {
	case game.PhaseWhiteSum:
		moves = m.gameState.GetValidMovesPhase1(m.playerID)
	case game.PhaseColorCombo:
		moves = m.gameState.GetValidMovesPhase2(m.playerID)
	}

	for _, mv := range moves {
		if m.validMoves[mv.Color] == nil {
			m.validMoves[mv.Color] = make(map[int]bool)
		}
		m.validMoves[mv.Color][mv.Number] = true
	}

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

// RefreshFromGame refreshes moves and submission state from the current game state.
func (m *BoardModel) RefreshFromGame() {
	m.refreshMoves()
}

// SetStatusMsg sets a status message (displayed as error).
func (m *BoardModel) SetStatusMsg(msg string) {
	m.statusMsg = msg
}

// AddMessage adds a notification message.
func (m *BoardModel) AddMessage(msg string) {
	m.messages = append(m.messages, msg)
	if len(m.messages) > 10 {
		m.messages = m.messages[len(m.messages)-10:]
	}
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
