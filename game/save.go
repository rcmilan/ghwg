package game

// 08 W3 (P1) seam, owned by P1-S: the save. Only a Game with persist set reads or writes it.

// saveData is what the save holds: the prestige data, plus the last dawn checkpoint (nil right after a prestige
// until the next dawn... or the Run's start state; P1-S decides and documents).
type saveData struct {
	Level int       `json:"level"`
	Bonus float64   `json:"bonus"`
	Run   *runState `json:"run,omitempty"`
}

// runState: the dawn checkpoint. P1-S fills its fields.
type runState struct{}

// saveGame writes the save now (at dawn, and right after a prestige). Stub: does nothing.
func (g *Game) saveGame() {}
