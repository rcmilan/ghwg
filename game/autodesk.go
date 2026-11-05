package game

import (
	"image"

	"shift/world"
)

// atPanel: the Room whose Door Panel the Concierge stands at (04 D113), -1 if none.
func (g *Game) atPanel() int {
	at := image.Pt(int(g.player.X), int(g.player.Y)).Div(world.TileSize)
	for i, r := range g.hotel.Rooms {
		if r.PanelSpot().Div(world.TileSize) == at {
			return i
		}
	}
	return -1
}

// autoBook (D115, D118): the first Party standing at the counter, in arrival order, that a Vacant Auto
// Room fits is booked into the one paying it most per night (ties: the lowest Room number), for 85%.
// Again until none fits, so no fitting Party is left waiting a frame.
func (g *Game) autoBook() {
	for booked := true; booked; {
		booked = false
		for _, p := range g.queue {
			best := -1
			for i, r := range g.hotel.Rooms {
				if !g.autoRoom[i] || g.fitsParty(p, i) != "" {
					continue
				}
				if b := g.hotel.Rooms[max(best, 0)]; best < 0 || p.rateIn(r) > p.rateIn(b) || p.rateIn(r) == p.rateIn(b) && r.Number < b.Number {
					best = i
				}
			}
			if best >= 0 {
				g.bookParty(p, best, true)
				booked = true
				break
			}
		}
	}
}
