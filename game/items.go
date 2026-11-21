package game

// 08 W5-ITEMS: Morwenna's six items, owned for good once a beat first gives them (itemOn, quest.go). Stubs from
// the W5 seam.

// itemRun: the items' Run state (where Gerald and the Ward are, the Storm jar used today), in the checkpoint.
type itemRun struct{}

// stormJarOn: the Storm jar's HOTEL row is on sale now (owned, not used today).
func (g *Game) stormJarOn() bool { return false }

// warded: the Ward is on Room room: its human Parties never flee.
func (g *Game) warded(room int) bool { return false }

// conciergeBoost: the Concierge's walking speed multiplier (the Broom: faster in ghost).
func (g *Game) conciergeBoost() float64 { return 1 }

// itemActors: the Cat and Gerald for the y-sorted pass.
func (g *Game) itemActors() []actor { return nil }

// updateItems runs each step of world time.
func (g *Game) updateItems() {}
