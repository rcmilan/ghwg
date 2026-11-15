package game

import "ghosthotel/world"

// Corridor mirrors (08 B1): each Corridor a New floor made has a Mirror on a random Room's front wall, near
// a corner, chosen per Run (the seeded rng) and covered at first. Uncovered, it shifts like the Lobby Mirror,
// from the Corridor cell below it. Ectoplasm uncovers it, by SPACE on that cell or the HOTEL row, each one
// costing twice the last. The state lives on the Hotel (world.CorridorMirror), which a Run's start rebuilds.
const (
	CorridorMirror Unlock = "corridor-mirror" // the HOTEL row that uncovers the next one; not an unlock, its ID only names it
	uncoverEcto           = 40                // Ectoplasm: the first uncovered (placeholder); each next one costs twice the last
)

// corridorMirrors: the Mirrors the rebuilt hotel h hangs: one for each floor a growth made, the old ones kept
// as they were (corner, and whether uncovered), a new floor's rolled now.
func (g *Game) corridorMirrors(old, h *world.Hotel) []world.CorridorMirror {
	cols, rows := h.Grid()
	var ms []world.CorridorMirror
	for f := g.grid0[1]; f < rows; f++ { // the floors from the ground up: those the Run began with have none
		m := world.CorridorMirror{Floor: f, Col: g.rng.IntN(cols), West: g.rng.IntN(2) == 0}
		for _, o := range old.Corridors {
			if o.Floor == f {
				m = o
			}
		}
		ms = append(ms, m)
	}
	return ms
}

// atCorridorMirror: the Corridor Mirror whose cell the Concierge stands on, or -1.
func (g *Game) atCorridorMirror() int {
	for k, m := range g.hotel.Corridors {
		if g.feet().In(m.Spot) {
			return k
		}
	}
	return -1
}

// uncoverCost: Ectoplasm to uncover the next: uncoverEcto, doubled for each already uncovered.
func uncoverCost(g *Game) Cost {
	n := Num(uncoverEcto)
	for _, m := range g.hotel.Corridors {
		if m.Open {
			n *= 2
		}
	}
	return Cost{Ecto: n}
}

// nextCovered: the covered Mirror the HOTEL row uncovers, the lowest floor's; -1 if none.
func (g *Game) nextCovered() int {
	for k, m := range g.hotel.Corridors {
		if !m.Open {
			return k
		}
	}
	return -1
}

// uncover opens Corridor Mirror k, paying for it; false if the wallet can't.
func (g *Game) uncover(k int) bool {
	if g.hotel.Corridors[k].Open || !g.pay(uncoverCost(g)) {
		return false
	}
	g.hotel.Corridors[k].Open = true
	return true
}

// coveredCorridor: the prop is a tile of a Corridor Mirror still covered.
func (g *Game) coveredCorridor(p prop) bool {
	for _, m := range g.hotel.Corridors {
		if !m.Open && p.cx == m.Cell.X && (p.cy == m.Cell.Y || p.cy == m.Cell.Y+1) {
			return true
		}
	}
	return false
}

// DevCorridors is main's -corridors: the first n Corridor Mirrors are uncovered, and the Concierge stands on
// the cell beside the first covered one's (the first's if all are), with the Ectoplasm to uncover it (use with -grow floor).
func (g *Game) DevCorridors(n int) {
	for k := range g.hotel.Corridors {
		g.hotel.Corridors[k].Open = k < n
	}
	if len(g.hotel.Corridors) == 0 {
		return
	}
	g.wallet[Ecto] = max(g.wallet[Ecto], 2*uncoverEcto)
	s := g.hotel.Corridors[max(0, g.nextCovered())].Spot
	g.player.X, g.player.Y = float64(s.Max.X+world.TileSize/2), float64(s.Min.Y+world.TileSize/2) // a step east: its glow shows once he stands on it
	w, h := g.mapSize()
	g.cam.Snap(g.player.X, g.player.Y, w, h)
}
