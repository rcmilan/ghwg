package game

import (
	"slices"

	"ghosthotel/entities"
)

// 08 W4 (T373 seam, T376 QUIRK): the quirk rules of context/08-quests/cast.md, as hooks at the places the economy
// acts. Every body returns the neutral value (x1, true) until QUIRK fills it; the signatures are meant to be
// enough for every rule, so QUIRK edits bodies only. A multiplier combines with its upgrade's at the call site.

// A Party of two can hold two quirks. Rules for a pair: Big tipper, Night owl and Neat freak work for the whole
// Party if either guest has it (a Party tips, sleeps and lives in one Room); Medium and Skeptic are per guest (each
// sighting is one guest's, a flee is one guest's); a ghost pair multiplies what each ghost's quirk does to the Room.

// hasQuirk: some guest of the Party has the quirk.
func hasQuirk(p *Party, q Quirk) bool {
	return p != nil && slices.ContainsFunc(p.Guests, func(guest *entities.Guest) bool { return guest.Who.Quirk == q })
}

// ghostParty: the ghost Party booked into the Room, nil when none.
func (g *Game) ghostParty(room int) *Party {
	for _, p := range g.parties {
		if p.Occupies(room, Ghost) && p.state != out {
			return p
		}
	}
	return nil
}

// hauntQuirkMul: the Mood a ghost's haunt costs its Room, times: Gentle 0; Jealous x2 when the Room holds a Party
// of two; a Neat freak's Room feels the spiritual dirt it leaves twice (dirtQuirkMul).
func (g *Game) hauntQuirkMul(room int, ghost *entities.Guest) float64 {
	m := 1.0
	switch ghost.Who.Quirk {
	case Gentle:
		return 0
	case Jealous:
		if p, _ := g.insideIn(room, Human); p != nil && len(p.Guests) == 2 {
			m = 2
		}
	}
	p, _ := g.insideIn(room, Human)
	return m * g.dirtQuirkMul(p)
}

// sightingQuirkMul: the Ectoplasm a sighting by human `human` of Party p pays, times: Medium x2; Skeptic 0; the
// Room's ghost Show-off x2 or Gentle x1/2 (each ghost of a pair).
func (g *Game) sightingQuirkMul(p *Party, human *entities.Guest) float64 {
	switch human.Who.Quirk {
	case Skeptic:
		return 0
	case Medium:
		return 2 * g.ghostSightingMul(p.Room)
	}
	return g.ghostSightingMul(p.Room)
}

// ghostSightingMul: what the Room's ghost quirk does to a sighting's Ectoplasm.
func (g *Game) ghostSightingMul(room int) float64 {
	m := 1.0
	if gp := g.ghostParty(room); gp != nil {
		for _, ghost := range gp.Guests {
			switch ghost.Who.Quirk {
			case ShowOff:
				m *= 2
			case Gentle:
				m /= 2
			}
		}
	}
	return m
}

// hauntGapQuirkMul: a ghost's gap between haunts, times: Show-off x1/2.
func (g *Game) hauntGapQuirkMul(ghost *entities.Guest) float64 {
	if ghost.Who.Quirk == ShowOff {
		return 0.5
	}
	return 1
}

// tipQuirkMul: the checkout tip of Party p, times: Big tipper x2.
func (g *Game) tipQuirkMul(p *Party) float64 {
	if hasQuirk(p, BigTipper) {
		return 2
	}
	return 1
}

// dirtQuirkMul: how hard the dirt of Party p's stay drags its Room's Mood, times: Neat freak x2. It lands where the
// Room's Mood falls during the stay (a haunt, a fright), not at checkout, which is the next guest's Mood.
func (g *Game) dirtQuirkMul(p *Party) float64 {
	if hasQuirk(p, NeatFreak) {
		return 2
	}
	return 1
}

// mayFlee: Party p's guest at its FleeAt steps out, then flees; false and it never does: Skeptic (that guest only).
func (g *Game) mayFlee(p *Party, guest *entities.Guest) bool { return guest.Who.Quirk != Skeptic }

// sleeps: Party p goes to bed at night and asks for nothing then; false and it stays up and asks at night too: Night owl.
func (g *Game) sleeps(p *Party) bool { return !hasQuirk(p, NightOwl) }

// mayHaunt: the ghost may haunt its Room now; false: Shy, while a human in its Room is awake.
func (g *Game) mayHaunt(room int, ghost *entities.Guest) bool {
	if ghost.Who.Quirk != Shy {
		return true
	}
	p, in := g.insideIn(room, Human)
	return len(in) == 0 || asleep(g.clock) && g.sleeps(p)
}

// mayPass: ghost Party p may pass on by itself once spent, and leave at its Checkout; false: Homebody (either of a
// pair), until Father M sends it over (sendOver sets PassAt, which this does not gate).
func (g *Game) mayPass(p *Party) bool { return !hasQuirk(p, Homebody) }

// hauntGap: the ghost's game minutes between haunts, 0 for one that does not haunt (Haunt of 0).
func (g *Game) hauntGap(ghost *entities.Guest) int {
	if ghost.Haunt <= 0 {
		return 0
	}
	return max(1, scaleInt(ghost.Haunt, g.hauntGapQuirkMul(ghost)))
}
