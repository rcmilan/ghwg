package game

import (
	"image"

	"ghosthotel/entities"
)

// 08 W2 (B1) seams: the shared names its sub-slices call each other through (context/08-quests/plan.md).

// staffSpeed: every staff member's walking speed multiplier (running shoes); not the Concierge's. B1-S fills it.
func (g *Game) staffSpeed() float64 { return 1 + shoeBoost*float64(g.shoeTier()) }

// sendOver: Father M's ritual sends every ghost in the Room over now (W2 gaps grill), both slots of a mirror
// Room, each paying at once the Ectoplasm its remaining haunts would have earned. B1-P calls it.
func (g *Game) sendOver(room int) {
	for _, p := range g.parties {
		if p.Realm != Ghost || p.Room != room || p.state != staying || p.Guests[0].PassAt != 0 || g.questPinned(p) { // 08 W5: not a quest ghost
			continue
		}
		p.banished = true
		for _, guest := range p.Guests {
			if left := max(1, guest.Phantasm) - guest.Haunted; left > 0 {
				g.earn(Ecto, Num(left)*hauntEcto(guest), image.Pt(int(guest.X), int(guest.Y)))
			}
			guest.PassAt, guest.Path = g.clock, nil
		}
	}
}

// hauntEcto: the Ectoplasm one of the ghost's haunts would have earned: the spiritual dirt it leaves in the Room
// (its Mood damage, 4 Mood points a 3 dirt) cleaned in ghost, where harvestRate of it is banked (dirt.go).
// A placeholder until W2's sim.
func hauntEcto(ghost *entities.Guest) Num { return Num(ghostHauntDamage(ghost)) * 3 / 4 * harvestRate }
