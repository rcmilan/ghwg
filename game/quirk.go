package game

import "ghosthotel/entities"

// 08 W4 (T373 seam, T376 QUIRK): the quirk rules of context/08-quests/cast.md, as hooks at the places the economy
// acts. Every body returns the neutral value (x1, true) until QUIRK fills it; the signatures are meant to be
// enough for every rule, so QUIRK edits bodies only. A multiplier combines with its upgrade's at the call site.

// hauntQuirkMul: the Mood a ghost's haunt costs its Room, times: Gentle 0; Jealous x2 when the Room holds a Party of two.
func (g *Game) hauntQuirkMul(room int, ghost *entities.Guest) float64 { return 1 }

// sightingQuirkMul: the Ectoplasm a sighting by human `human` of Party p pays, times: Medium x2; Skeptic 0; the
// Room's ghost Show-off x2 or Gentle x1/2.
func (g *Game) sightingQuirkMul(p *Party, human *entities.Guest) float64 { return 1 }

// hauntGapQuirkMul: a ghost's gap between haunts, times: Show-off x1/2.
func (g *Game) hauntGapQuirkMul(ghost *entities.Guest) float64 { return 1 }

// tipQuirkMul: the checkout tip of Party p, times: Big tipper x2.
func (g *Game) tipQuirkMul(p *Party) float64 { return 1 }

// dirtQuirkMul: how hard the dirt of Party p's stay drags its Room's Mood, times: Neat freak x2.
func (g *Game) dirtQuirkMul(p *Party) float64 { return 1 }

// mayFlee: Party p's guest at its FleeAt steps out, then flees; false and it never does: Skeptic.
func (g *Game) mayFlee(p *Party, guest *entities.Guest) bool { return true }

// sleeps: Party p goes to bed at night and asks for nothing then; false and it stays up and asks at night too: Night owl.
func (g *Game) sleeps(p *Party) bool { return true }

// mayHaunt: the ghost may haunt its Room now; false: Shy, while a human in its Room is awake.
func (g *Game) mayHaunt(room int, ghost *entities.Guest) bool { return true }

// mayPass: ghost Party p may pass on by itself once spent; false: Homebody, until Father M sends it over.
func (g *Game) mayPass(p *Party) bool { return true }

// hauntGap: the ghost's game minutes between haunts, 0 for one that does not haunt (Haunt of 0).
func (g *Game) hauntGap(ghost *entities.Guest) int {
	if ghost.Haunt <= 0 {
		return 0
	}
	return max(1, scaleInt(ghost.Haunt, g.hauntGapQuirkMul(ghost)))
}
