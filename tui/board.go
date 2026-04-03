package tui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/lucasacastro/qwixx/game"
)

// AutoPassMsg is sent after a delay to auto-pass when no valid moves exist.
type AutoPassMsg struct{}

type cursor struct {
	row int // 0=Red, 1=Yellow, 2=Green, 3=Blue
	col int // 0-10 (index into the 11 numbers per row)
}

// BoardModel handles the main game board screen.
type BoardModel struct {
	playerID   string
	nickname   string
	gameState  *game.Game
	validMoves map[game.Color]map[int]bool
	cursor     cursor
	statusMsg  string
	messages   []string

	// Player marks cache (for cursor skip logic)
	playerMarks map[game.Color]map[int]bool

	// Action tracking for the app layer
	hasAction  bool
	actionMove *game.Move // nil = pass

	// Auto-pass state
	autoPassPending bool

	// Dice rolling animation
	diceAnim DiceAnimation
}

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
	case AutoPassMsg:
		if m.autoPassPending {
			m.autoPassPending = false
			m.hasAction = true
			m.actionMove = nil
		}
		return m, nil
	case DiceAnimTickMsg:
		if m.diceAnim.IsRunning() {
			m.diceAnim.Tick()
			return m, m.diceAnim.TickCmd()
		}
		return m, nil
	}
	return m, nil
}

func (m BoardModel) handleKey(msg tea.KeyMsg) (BoardModel, tea.Cmd) {
	switch msg.Type {
	case tea.KeyCtrlC:
		return m, tea.Quit
	}

	m.statusMsg = ""

	step := m.gameState.GetPlayerStep(m.playerID)
	if step == game.StepWaiting || step == game.StepDone {
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
			m.cursor.col = m.findNearestUnmarked(m.cursor.row, m.cursor.col, rows)
		}
	case "down", "j":
		if m.cursor.row < 3 {
			m.cursor.row++
			m.cursor.col = m.findNearestUnmarked(m.cursor.row, m.cursor.col, rows)
		}
	case "left", "h":
		m.cursor.col = m.findNextUnmarked(m.cursor.row, m.cursor.col, -1, rows)
	case "right", "l":
		m.cursor.col = m.findNextUnmarked(m.cursor.row, m.cursor.col, 1, rows)
	case "enter", " ":
		c := colors[m.cursor.row]
		num := rows[m.cursor.row][m.cursor.col]
		if m.validMoves[c] != nil && m.validMoves[c][num] {
			m.hasAction = true
			m.actionMove = &game.Move{Color: c, Number: num}
		}
	case "p", "P":
		m.hasAction = true
		m.actionMove = nil
	}

	return m, nil
}

// isMarkedAt checks if the cell at the given row/col is marked.
func (m BoardModel) isMarkedAt(row, col int, rows [][]int) bool {
	if m.playerMarks == nil {
		return false
	}
	c := game.AllColors[row]
	num := rows[row][col]
	return m.playerMarks[c][num]
}

// findNextUnmarked finds the next unmarked cell in the given direction (dir: -1 or +1).
// Returns the new column index. If no unmarked cell exists in that direction, stays put.
func (m BoardModel) findNextUnmarked(row, col, dir int, rows [][]int) int {
	maxCol := len(rows[row]) - 1
	next := col + dir
	for next >= 0 && next <= maxCol {
		if !m.isMarkedAt(row, next, rows) {
			return next
		}
		next += dir
	}
	// No unmarked cell found in that direction, stay put
	return col
}

// findNearestUnmarked finds the nearest unmarked cell to the target column on the given row.
// Searches outward from targetCol (right first, then left). If none found, returns targetCol.
func (m BoardModel) findNearestUnmarked(row, targetCol int, rows [][]int) int {
	maxCol := len(rows[row]) - 1
	if targetCol > maxCol {
		targetCol = maxCol
	}
	if !m.isMarkedAt(row, targetCol, rows) {
		return targetCol
	}
	// Search outward from targetCol
	for offset := 1; offset <= maxCol; offset++ {
		right := targetCol + offset
		if right <= maxCol && !m.isMarkedAt(row, right, rows) {
			return right
		}
		left := targetCol - offset
		if left >= 0 && !m.isMarkedAt(row, left, rows) {
			return left
		}
	}
	// All cells marked, just stay at targetCol
	return targetCol
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
		start := len(m.messages) - 3
		if start < 0 {
			start = 0
		}
		for _, msg := range m.messages[start:] {
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
	turnNum := g.GetTurnNumber()
	step := g.GetPlayerStep(m.playerID)
	isActive := g.IsActivePlayer(m.playerID)

	var stepDesc string
	switch step {
	case game.StepWhite:
		stepDesc = "Mark the white sum or pass"
	case game.StepColor:
		stepDesc = "Mark a color combo or pass"
	case game.StepWaiting, game.StepDone:
		stepDesc = "Waiting for others..."
	}

	roller := fmt.Sprintf(" %s rolled ", activeNick)
	if isActive {
		roller = " You rolled "
	}

	return lipgloss.JoinHorizontal(lipgloss.Center,
		TitleStyle.Render(fmt.Sprintf(" Turn %d ", turnNum)),
		"  ",
		HighlightStyle.Render(roller),
		"  ",
		SubtitleStyle.Render(stepDesc),
	)
}

func (m BoardModel) renderDice() string {
	roll := m.gameState.GetCurrentRollSnapshot()
	if roll == nil {
		return SubtitleStyle.Render("  Rolling dice...")
	}

	animating := m.diceAnim.IsRunning()

	// Helper to render a single die with the right style
	renderDie := func(style lipgloss.Style, key diceKey, finalVal int) string {
		if animating {
			val := m.diceAnim.DisplayValue(key)
			if m.diceAnim.IsSettled(key) {
				return style.Render(fmt.Sprintf(" %d ", val))
			}
			return DiceRollingStyle.Inherit(style).Render(fmt.Sprintf(" %d ", val))
		}
		return style.Render(fmt.Sprintf(" %d ", finalVal))
	}

	var dice []string
	dice = append(dice, renderDie(WhiteDieStyle, dkWhite1, roll.White1))
	dice = append(dice, renderDie(WhiteDieStyle, dkWhite2, roll.White2))

	if roll.ActiveColors[game.Red] {
		dice = append(dice, renderDie(RedDieStyle, dkRed, roll.Red))
	}
	if roll.ActiveColors[game.Yellow] {
		dice = append(dice, renderDie(YellowDieStyle, dkYellow, roll.Yellow))
	}
	if roll.ActiveColors[game.Green] {
		dice = append(dice, renderDie(GreenDieStyle, dkGreen, roll.Green))
	}
	if roll.ActiveColors[game.Blue] {
		dice = append(dice, renderDie(BlueDieStyle, dkBlue, roll.Blue))
	}

	diceRow := "  Dice:  " + strings.Join(dice, "  ")

	if animating {
		return diceRow + SubtitleStyle.Render("      Rolling...")
	}

	step := m.gameState.GetPlayerStep(m.playerID)
	isActive := m.gameState.IsActivePlayer(m.playerID)

	whiteSumStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FFFFFF")).Background(lipgloss.Color("#555555")).Padding(0, 1)
	whiteSum := fmt.Sprintf("      White sum = %s", whiteSumStyle.Render(fmt.Sprintf(" %d ", roll.WhiteSum())))

	// Show what this step means
	var hint string
	switch step {
	case game.StepWhite:
		hint = fmt.Sprintf("\n  %s", SubtitleStyle.Render(fmt.Sprintf("You can mark %d on any row", roll.WhiteSum())))
	case game.StepColor:
		if isActive {
			hint = fmt.Sprintf("\n  %s", SubtitleStyle.Render("You can mark a white + colored die combo"))
		}
	}

	return diceRow + whiteSum + hint
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

	step := m.gameState.GetPlayerStep(m.playerID)
	canAct := step == game.StepWhite || step == game.StepColor

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

	label := colorStyle.Render(fmt.Sprintf("  %-6s", c.String()))

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
			numStrs = append(numStrs, CursorValidStyle.Render(numStr))
		} else if isCursor {
			numStrs = append(numStrs, CursorInvalidStyle.Render(numStr))
		} else if isValid {
			numStrs = append(numStrs, ValidMoveStyle.Render(numStr))
		} else {
			numStrs = append(numStrs, DimStyle.Render(numStr))
		}
	}

	lockStr := ""
	if isLocked {
		lockStr = "  " + LockedStyle.Render("LOCKED")
	}

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

		// Show check/waiting status
		step := m.gameState.GetPlayerStep(p.ID)
		var statusIcon string
		if step == game.StepWaiting || step == game.StepDone {
			statusIcon = SuccessStyle.Render(" ✓")
		}

		if p.ID == activeID {
			parts = append(parts, HighlightStyle.Render(name)+" "+SubtitleStyle.Render("(roller)")+penStr+statusIcon)
		} else {
			parts = append(parts, SubtitleStyle.Render(name)+penStr+statusIcon)
		}
	}

	return "  Players: " + strings.Join(parts, "  |  ")
}

func (m BoardModel) renderControls() string {
	step := m.gameState.GetPlayerStep(m.playerID)

	switch step {
	case game.StepWhite:
		return SubtitleStyle.Render("  Arrows to move  |  Enter to mark white sum  |  P to pass")
	case game.StepColor:
		return SubtitleStyle.Render("  Arrows to move  |  Enter to mark color combo  |  P to pass")
	case game.StepWaiting, game.StepDone:
		return StatusStyle.Render("  Waiting for other players...")
	default:
		return ""
	}
}

// HasAction returns true if the player performed an action (mark or pass).
func (m BoardModel) HasAction() bool {
	return m.hasAction
}

// ActionMove returns the action move (nil = pass).
func (m BoardModel) ActionMove() *game.Move {
	return m.actionMove
}

// ConsumeAction clears the pending action.
func (m *BoardModel) ConsumeAction() {
	m.hasAction = false
	m.actionMove = nil
}

// RefreshFromGame refreshes valid moves from the current game state.
func (m *BoardModel) RefreshFromGame() {
	moves := m.gameState.GetValidMoves(m.playerID)
	m.validMoves = make(map[game.Color]map[int]bool)
	for _, mv := range moves {
		if m.validMoves[mv.Color] == nil {
			m.validMoves[mv.Color] = make(map[int]bool)
		}
		m.validMoves[mv.Color][mv.Number] = true
	}
	m.hasAction = false
	m.actionMove = nil

	// Cache player marks for cursor skip logic
	snapshots := m.gameState.GetPlayerSnapshots()
	for _, s := range snapshots {
		if s.ID == m.playerID {
			m.playerMarks = s.Marks
			break
		}
	}

	// Check for auto-pass: if player has an actionable step but no valid moves
	step := m.gameState.GetPlayerStep(m.playerID)
	if (step == game.StepWhite || step == game.StepColor) && len(moves) == 0 {
		m.autoPassPending = true
		m.AddMessage("No valid moves — auto-passing...")
	} else {
		m.autoPassPending = false
	}
}

// AutoPassCmd returns a tea.Cmd for the auto-pass delay if one is pending, or nil.
func (m *BoardModel) AutoPassCmd() tea.Cmd {
	if !m.autoPassPending {
		return nil
	}
	return tea.Tick(1*time.Second, func(time.Time) tea.Msg {
		return AutoPassMsg{}
	})
}

// StartDiceAnimation starts the dice rolling animation for the given roll.
func (m *BoardModel) StartDiceAnimation(roll *game.DiceRollSnapshot) tea.Cmd {
	m.diceAnim = NewDiceAnimation(roll)
	return m.diceAnim.TickCmd()
}

// SetStatusMsg sets a status message.
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
