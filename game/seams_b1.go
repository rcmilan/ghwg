package game

// 08 W2 (B1) seams: the shared names its sub-slices call each other through (context/08-quests/plan.md).

// staffSpeed: every staff member's walking speed multiplier (running shoes); not the Concierge's. B1-S fills it.
func (g *Game) staffSpeed() float64 { return 1 + shoeBoost*float64(g.shoeTier()) }

// sendOver: Father M's ritual sends every ghost in the Room over now (W2 gaps grill), each paying the Ecto its
// remaining haunts would have earned. B1-P calls it; B1-M fills it (the mirror's second ghost, the reward).
// Until then: the Room's staying ghosts start passing on, no reward.
func (g *Game) sendOver(room int) {
	for _, p := range g.parties {
		if p.Realm == Ghost && p.Room == room && p.state == staying && p.Guests[0].PassAt == 0 {
			for _, guest := range p.Guests {
				guest.PassAt, guest.Path = g.clock, nil
			}
		}
	}
}
