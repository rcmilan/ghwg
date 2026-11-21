package game

// 08 W5-VERBS: the objective verbs (TELL, TAKE and GIVE on the Card, the ghost Request, the talk's target
// override, the quest rows, the quest guest book entries, the spirit with no ghost, REVEAL's quest facts, Father
// K's retirement party). Each calls questEvent. Stubs from the W5 seam.

// verbState: the verbs' Run state (the word or quest thing the Concierge holds, the spirit, ...).
type verbState struct{}

// questRowOn: id is a quest row of the Terminal (quest true); on: it is on sale now.
func (g *Game) questRowOn(id Unlock) (on, quest bool) {
	switch id {
	case rowWreath, rowCrate:
		return false, true
	case rowStorm:
		return g.stormJarOn(), true
	}
	return false, false
}

// verbActors: the verbs' actors for the y-sorted pass (the spirit).
func (g *Game) verbActors() []actor { return nil }

// updateVerbs runs each step of world time.
func (g *Game) updateVerbs() {}

// The quest rows' IDs (shop.go).
const (
	rowWreath Unlock = "quest-wreath"
	rowCrate  Unlock = "quest-crate"
	rowStorm  Unlock = "item-storm"
)
