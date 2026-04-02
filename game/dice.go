package game

import (
	"math/rand"
)

// Die represents a single die with a color and a value.
type Die struct {
	Color Color  // Color of the die (-1 for white)
	White bool   // True if this is a white die
	Value int    // Current face value (1-6)
}

// DiceRoll represents the result of rolling all active dice.
type DiceRoll struct {
	White1 int
	White2 int
	Red    int
	Yellow int
	Green  int
	Blue   int

	// Which colored dice are still active (not removed due to locked rows)
	ActiveColors map[Color]bool
}

// NewDiceRoll creates a fresh dice roll with all colors active.
func NewDiceRoll(lockedRows map[Color]string) *DiceRoll {
	active := make(map[Color]bool)
	for _, c := range AllColors {
		if _, locked := lockedRows[c]; !locked {
			active[c] = true
		}
	}
	return &DiceRoll{
		ActiveColors: active,
	}
}

// Roll rolls all active dice.
func (d *DiceRoll) Roll(rng *rand.Rand) {
	d.White1 = rng.Intn(6) + 1
	d.White2 = rng.Intn(6) + 1

	if d.ActiveColors[Red] {
		d.Red = rng.Intn(6) + 1
	}
	if d.ActiveColors[Yellow] {
		d.Yellow = rng.Intn(6) + 1
	}
	if d.ActiveColors[Green] {
		d.Green = rng.Intn(6) + 1
	}
	if d.ActiveColors[Blue] {
		d.Blue = rng.Intn(6) + 1
	}
}

// WhiteSum returns the sum of the two white dice.
func (d *DiceRoll) WhiteSum() int {
	return d.White1 + d.White2
}

// ColoredValue returns the value of a colored die.
func (d *DiceRoll) ColoredValue(c Color) int {
	switch c {
	case Red:
		return d.Red
	case Yellow:
		return d.Yellow
	case Green:
		return d.Green
	case Blue:
		return d.Blue
	default:
		return 0
	}
}

// ColorCombos returns all possible white+colored die sums for the active player.
// Each combo is a (Color, sum) pair.
func (d *DiceRoll) ColorCombos() []ColorCombo {
	var combos []ColorCombo
	for _, c := range AllColors {
		if !d.ActiveColors[c] {
			continue
		}
		colored := d.ColoredValue(c)
		// Can combine with either white die
		sum1 := d.White1 + colored
		sum2 := d.White2 + colored
		combos = append(combos, ColorCombo{Color: c, Sum: sum1, WhiteDie: 1})
		if sum1 != sum2 {
			combos = append(combos, ColorCombo{Color: c, Sum: sum2, WhiteDie: 2})
		}
	}
	return combos
}

// ColorCombo represents a possible white+colored die combination.
type ColorCombo struct {
	Color    Color
	Sum      int
	WhiteDie int // Which white die (1 or 2) was used
}
