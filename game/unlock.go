package game

// Unlock is a feature bought at the Terminal (D178). Later slices append their own.
type Unlock string

const (
	Mirror      Unlock = "mirror"
	SupplyWater Unlock = "supply-water"
	SupplyFood  Unlock = "supply-food"
	SupplyTowel Unlock = "supply-towel"
	Housekeep   Unlock = "housekeep"
	RoomLock    Unlock = "room-lock"
)

// unlocked: g.locked's zero value locks nothing, so the game plays as before until O1 sets a start state.
func (g *Game) unlocked(id Unlock) bool { return !g.locked[id] }
func (g *Game) unlock(id Unlock)        { delete(g.locked, id) }
