package tui

import (
	"fmt"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/lucasacastro/qwixx/game"
)

// ResultsModel handles the end-of-game results screen.
type ResultsModel struct {
	scores     []game.PlayerScore
	reason     string
	isCreator  bool
	playerID   string
	playAgain  bool
	quit       bool
}

// NewResultsModel creates a new results screen.
func NewResultsModel(scores []game.PlayerScore, reason string, isCreator bool, playerID string) ResultsModel {
	// Sort by total score descending (stable sort for consistent tie-breaking)
	sort.SliceStable(scores, func(i, j int) bool {
		return scores[i].Total > scores[j].Total
	})

	return ResultsModel{
		scores:    scores,
		reason:    reason,
		isCreator: isCreator,
		playerID:  playerID,
	}
}

func (m ResultsModel) Init() tea.Cmd {
	return nil
}

func (m ResultsModel) Update(msg tea.Msg) (ResultsModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "enter":
			if m.isCreator {
				m.playAgain = true
			}
		case "q", "Q":
			m.quit = true
			return m, tea.Quit
		case "ctrl+c":
			return m, tea.Quit
		}
	}

	return m, nil
}

func (m ResultsModel) View() string {
	var b strings.Builder

	b.WriteString("\n")
	b.WriteString(TitleStyle.Render(" GAME OVER "))
	b.WriteString("\n\n")

	b.WriteString(StatusStyle.Render(m.reason))
	b.WriteString("\n\n")

	// Column headers
	header := fmt.Sprintf("  %-12s %5s %5s %5s %5s %5s %5s",
		"Player", "Red", "Yel", "Grn", "Blu", "Pen", "TOTAL")
	b.WriteString(SubtitleStyle.Render(header))
	b.WriteString("\n")
	b.WriteString(SubtitleStyle.Render("  " + strings.Repeat("-", 52)))
	b.WriteString("\n")

	// Player rows
	for i, score := range m.scores {
		name := score.Nickname
		if len(name) > 10 {
			name = name[:10]
		}
		if score.PlayerID == m.playerID {
			name += " (you)"
		}

		penaltyStr := fmt.Sprintf("-%d", score.Penalties*5)
		if score.Penalties == 0 {
			penaltyStr = "0"
		}

		line := fmt.Sprintf("  %-12s %5d %5d %5d %5d %5s %5d",
			name,
			score.RowScores[game.Red],
			score.RowScores[game.Yellow],
			score.RowScores[game.Green],
			score.RowScores[game.Blue],
			penaltyStr,
			score.Total,
		)

		if i == 0 {
			b.WriteString(WinnerStyle.Render(line))
			b.WriteString(WinnerStyle.Render(" ★"))
		} else if score.PlayerID == m.playerID {
			b.WriteString(HighlightStyle.Render(line))
		} else {
			b.WriteString(line)
		}
		b.WriteString("\n")
	}

	b.WriteString("\n")

	// Winner announcement
	if len(m.scores) > 0 {
		winner := m.scores[0]
		b.WriteString(WinnerStyle.Render(fmt.Sprintf("  %s wins with %d points!", winner.Nickname, winner.Total)))
		b.WriteString("\n\n")
	}

	// Actions
	if m.isCreator {
		b.WriteString(SuccessStyle.Render("  Press Enter to play again"))
		b.WriteString("  |  ")
		b.WriteString(SubtitleStyle.Render("Press Q to quit"))
	} else {
		b.WriteString(SubtitleStyle.Render("  Waiting for host..."))
		b.WriteString("  |  ")
		b.WriteString(SubtitleStyle.Render("Press Q to quit"))
	}

	return b.String()
}

// PlayAgain returns true if the creator wants to play again.
func (m ResultsModel) PlayAgain() bool {
	return m.playAgain
}

// Quit returns true if the player wants to quit.
func (m ResultsModel) Quit() bool {
	return m.quit
}
