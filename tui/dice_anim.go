package tui

import (
	"math/rand"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/lucasacastro/qwixx/game"
)

const (
	diceAnimDuration = 2500 * time.Millisecond
	diceAnimMinTick  = 50 * time.Millisecond
	diceAnimMaxTick  = 250 * time.Millisecond
)

// DiceAnimTickMsg is sent on each animation frame.
type DiceAnimTickMsg struct{}

// diceKey identifies a die in the animation.
type diceKey int

const (
	dkWhite1 diceKey = iota
	dkWhite2
	dkRed
	dkYellow
	dkGreen
	dkBlue
)

// DiceAnimation manages the dice rolling animation state.
type DiceAnimation struct {
	running     bool
	startTime   time.Time
	finalRoll   *game.DiceRollSnapshot
	displayVals map[diceKey]int
	settled     map[diceKey]bool
	settleOrder []diceKey
	rng         *rand.Rand
}

// NewDiceAnimation creates a new animation for the given roll.
func NewDiceAnimation(roll *game.DiceRollSnapshot) DiceAnimation {
	// Build the list of active dice
	order := []diceKey{dkWhite1, dkWhite2}
	if roll.ActiveColors[game.Red] {
		order = append(order, dkRed)
	}
	if roll.ActiveColors[game.Yellow] {
		order = append(order, dkYellow)
	}
	if roll.ActiveColors[game.Green] {
		order = append(order, dkGreen)
	}
	if roll.ActiveColors[game.Blue] {
		order = append(order, dkBlue)
	}

	// Shuffle settle order
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	shuffled := make([]diceKey, len(order))
	copy(shuffled, order)
	rng.Shuffle(len(shuffled), func(i, j int) {
		shuffled[i], shuffled[j] = shuffled[j], shuffled[i]
	})

	// Initialize display values with random numbers
	display := make(map[diceKey]int, len(order))
	for _, k := range order {
		display[k] = rng.Intn(6) + 1
	}

	return DiceAnimation{
		running:     true,
		startTime:   time.Now(),
		finalRoll:   roll,
		displayVals: display,
		settled:     make(map[diceKey]bool, len(order)),
		settleOrder: shuffled,
		rng:         rng,
	}
}

// Tick advances the animation state. Returns true if the animation is still running.
func (a *DiceAnimation) Tick() bool {
	if !a.running {
		return false
	}

	elapsed := time.Since(a.startTime)
	if elapsed >= diceAnimDuration {
		// Settle all remaining dice
		for _, k := range a.settleOrder {
			a.settled[k] = true
			a.displayVals[k] = a.finalValue(k)
		}
		a.running = false
		return false
	}

	// Determine how many dice should be settled based on elapsed time.
	// Dice start settling after 30% of the duration, evenly spaced.
	n := len(a.settleOrder)
	settleStart := diceAnimDuration * 30 / 100 // 30% mark
	settleWindow := diceAnimDuration - settleStart

	for i, k := range a.settleOrder {
		if a.settled[k] {
			continue
		}
		// Each die settles at: settleStart + (i+1)/n * settleWindow
		settleAt := settleStart + time.Duration(int64(settleWindow)*int64(i+1)/int64(n))
		if elapsed >= settleAt {
			a.settled[k] = true
			a.displayVals[k] = a.finalValue(k)
		}
	}

	// Randomize unsettled dice
	for _, k := range a.settleOrder {
		if !a.settled[k] {
			a.displayVals[k] = a.rng.Intn(6) + 1
		}
	}

	return true
}

// TickCmd returns the tea.Cmd for the next animation frame.
func (a *DiceAnimation) TickCmd() tea.Cmd {
	if !a.running {
		return nil
	}

	// Compute tick interval: starts fast, slows down as animation progresses
	elapsed := time.Since(a.startTime)
	progress := float64(elapsed) / float64(diceAnimDuration)
	if progress > 1 {
		progress = 1
	}
	// Ease-in: interval grows quadratically
	interval := diceAnimMinTick + time.Duration(float64(diceAnimMaxTick-diceAnimMinTick)*progress*progress)

	return tea.Tick(interval, func(time.Time) tea.Msg {
		return DiceAnimTickMsg{}
	})
}

// IsRunning returns true if the animation is still playing.
func (a *DiceAnimation) IsRunning() bool {
	return a.running
}

// IsSettled returns true if the given die has settled to its final value.
func (a *DiceAnimation) IsSettled(k diceKey) bool {
	return a.settled[k]
}

// DisplayValue returns the current display value for a die.
func (a *DiceAnimation) DisplayValue(k diceKey) int {
	if v, ok := a.displayVals[k]; ok {
		return v
	}
	return a.finalValue(k)
}

func (a *DiceAnimation) finalValue(k diceKey) int {
	switch k {
	case dkWhite1:
		return a.finalRoll.White1
	case dkWhite2:
		return a.finalRoll.White2
	case dkRed:
		return a.finalRoll.Red
	case dkYellow:
		return a.finalRoll.Yellow
	case dkGreen:
		return a.finalRoll.Green
	case dkBlue:
		return a.finalRoll.Blue
	default:
		return 1
	}
}
