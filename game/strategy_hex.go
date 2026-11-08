package game

func init() { Strategies["hexer"] = hexer{} }

// hexer stays in ghost and hexes Rooms with Guests inside from beside their walls, stopping short of
// fearLimit by two whole haunts: one to run on, one for the walk there (T192, 2026-11-08).
type hexer struct{}

func (hexer) Act(v View, p *Pilot) {
	switch i := hexRoom(v, fearLimit-2*hauntFear); {
	case v.Terminal.Open:
		p.Close()
	case !v.Ghost: // Fear shows only in ghost
		p.Flip(true)
	case i >= 0:
		p.Hex(i)
	}
}

// hexRoom: the Room to hex next, or -1: one with Guests inside, its Fear under most, the one the
// Concierge stands beside first (no walking between taps), else the least frightened.
func hexRoom(v View, most int) int {
	best := -1
	for i, r := range v.Rooms {
		if r.Vacant || r.Away || r.Fear >= most {
			continue
		}
		if beside(v.Hotel, v.Concierge) == i {
			return i
		}
		if best < 0 || r.Fear < v.Rooms[best].Fear {
			best = i
		}
	}
	return best
}
