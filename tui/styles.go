package tui

import "github.com/charmbracelet/lipgloss"

var (
	// Row colors
	RedStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("#FF4444")).Bold(true)
	YellowStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFDD44")).Bold(true)
	GreenStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#44DD44")).Bold(true)
	BlueStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#4488FF")).Bold(true)

	// Row background colors for marked numbers
	RedMarkedStyle    = lipgloss.NewStyle().Background(lipgloss.Color("#FF4444")).Foreground(lipgloss.Color("#000000")).Bold(true)
	YellowMarkedStyle = lipgloss.NewStyle().Background(lipgloss.Color("#FFDD44")).Foreground(lipgloss.Color("#000000")).Bold(true)
	GreenMarkedStyle  = lipgloss.NewStyle().Background(lipgloss.Color("#44DD44")).Foreground(lipgloss.Color("#000000")).Bold(true)
	BlueMarkedStyle   = lipgloss.NewStyle().Background(lipgloss.Color("#4488FF")).Foreground(lipgloss.Color("#000000")).Bold(true)

	// Dim style for unmarked numbers
	DimStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#666666"))

	// Locked row style
	LockedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#444444")).Strikethrough(true)

	// Dice styles
	WhiteDieStyle = lipgloss.NewStyle().
			Background(lipgloss.Color("#FFFFFF")).
			Foreground(lipgloss.Color("#000000")).
			Bold(true).
			Padding(0, 1)

	RedDieStyle = lipgloss.NewStyle().
			Background(lipgloss.Color("#FF4444")).
			Foreground(lipgloss.Color("#FFFFFF")).
			Bold(true).
			Padding(0, 1)

	YellowDieStyle = lipgloss.NewStyle().
			Background(lipgloss.Color("#FFDD44")).
			Foreground(lipgloss.Color("#000000")).
			Bold(true).
			Padding(0, 1)

	GreenDieStyle = lipgloss.NewStyle().
			Background(lipgloss.Color("#44DD44")).
			Foreground(lipgloss.Color("#000000")).
			Bold(true).
			Padding(0, 1)

	BlueDieStyle = lipgloss.NewStyle().
			Background(lipgloss.Color("#4488FF")).
			Foreground(lipgloss.Color("#FFFFFF")).
			Bold(true).
			Padding(0, 1)

	// Dice rolling animation style (dimmer to indicate motion)
	DiceRollingStyle = lipgloss.NewStyle().
				Faint(true)

	// UI elements
	TitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FFFFFF")).
			Background(lipgloss.Color("#7B2FBE")).
			Padding(0, 2)

	SubtitleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#AAAAAA"))

	HighlightStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FFFFFF"))

	ErrorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FF4444")).
			Bold(true)

	SuccessStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#44DD44")).
			Bold(true)

	PromptStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#7B2FBE")).
			Bold(true)

	BoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#7B2FBE")).
			Padding(1, 2)

	StatusStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFDD44")).
			Italic(true)

	PenaltyBoxEmpty  = lipgloss.NewStyle().Foreground(lipgloss.Color("#666666")).Render("[ ]")
	PenaltyBoxFilled = lipgloss.NewStyle().Foreground(lipgloss.Color("#FF4444")).Bold(true).Render("[X]")

	// Winner style
	WinnerStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FFDD44"))

	// Cursor style for valid move under cursor
	CursorValidStyle = lipgloss.NewStyle().
				Background(lipgloss.Color("#FFFFFF")).
				Foreground(lipgloss.Color("#000000")).
				Bold(true)

	// Cursor style for non-valid position
	CursorInvalidStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#888888")).
				Reverse(true)

	// Valid (selectable) number style
	ValidMoveStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFFFFF")).
			Bold(true).
			Underline(true)
)
