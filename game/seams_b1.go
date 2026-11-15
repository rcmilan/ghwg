package game

// 08 W2 (B1) seams: the shared names its sub-slices call each other through (context/08-quests/plan.md).

// staffSpeed: every staff member's walking speed multiplier (running shoes); not the Concierge's. B1-S fills it.
func (g *Game) staffSpeed() float64 { return 1 }
