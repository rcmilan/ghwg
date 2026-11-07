package game

import (
	"image"
	"math"
	"slices"

	"ghosthotel/entities"
	"ghosthotel/world"

	"github.com/hajimehoshi/ebiten/v2"
)

// The flee (D164) and hearing Guests (D165). Placeholders.
const (
	hopFrames    = 30 // a fleeing Guest's hop in place
	hopHeight    = 10 // world px at its top
	fleeSpeed    = 3 * departSpeed
	screamFrames = 90 // "AAAH!" shows this long from the flee
	hearRange    = 6  // tiles from the Concierge a Guest is heard
)

// lift: how high a fleeing Guest is in its hop, world px.
func (g *Game) lift(guest *entities.Guest) float64 {
	for _, p := range g.parties {
		if t := g.tick - p.fledAt; p.Fled && t < hopFrames && slices.Contains(p.Guests, guest) {
			return hopHeight * math.Sin(math.Pi*float64(t)/hopFrames)
		}
	}
	return 0
}

// hears (D165): the Concierge hears a Guest within hearRange; in real, not one inside a Room.
func (g *Game) hears(guest *entities.Guest) bool {
	if math.Hypot(guest.X-g.player.X, guest.Y-g.player.Y) > hearRange*world.TileSize {
		return false
	}
	at := image.Pt(int(guest.X), int(guest.Y))
	return g.ghost || !slices.ContainsFunc(g.hotel.Rooms, func(r world.Room) bool { return at.In(r.Shell()) })
}

// drawScreams: "AAAH!" by each fleeing Guest the Concierge hears, for screamFrames from its flee.
func (g *Game) drawScreams(s *ebiten.Image, ox, oy int) {
	for _, p := range g.parties {
		if !p.Fled || g.tick-p.fledAt >= screamFrames {
			continue
		}
		for _, guest := range p.Guests {
			if len(guest.Path) > 0 && g.hears(guest) { // no Path: gone out of the Exit
				g.say(s, "AAAH!", guest, ox, oy)
			}
		}
	}
}
