package game

import (
	"image"
	"math"
	"slices"

	"ghosthotel/entities"
	"ghosthotel/world"
)

// The flee (D164) and hearing Guests (D165). Placeholders.
const (
	hopFrames = 30 // a fleeing Guest's hop in place
	hopHeight = 10 // world px at its top
	fleeSpeed = 3 * departSpeed
	hearRange = 8 // tiles from the Concierge a Guest is heard: a whole Room from its door
)

// lift: how high a fleeing Guest is in its hop, or one startled by a Shadow (D169), world px.
func (g *Game) lift(guest *entities.Guest) float64 {
	for _, p := range g.parties {
		if t := g.now() - float64(p.fledAt); p.Fled && t < hopFrames && slices.Contains(p.Guests, guest) {
			return hopHeight * math.Sin(math.Pi*t/hopFrames)
		}
	}
	return g.startle(guest)
}

// hears (D165): the Concierge hears a Guest within hearRange; in real, with no wall between them, one
// outside the Rooms, or one in the Room he stands in (Z1: since K1 he walks in with its guests there).
func (g *Game) hears(guest *entities.Guest) bool {
	if math.Hypot(guest.X-g.player.X, guest.Y-g.player.Y) > hearRange*world.TileSize || g.realmOf(guest) != g.realm() {
		return false // a ghost is unheard in real; in ghost only ghosts are heard, not the living (B1 benevolent ghost)
	}
	return g.ghost || (!g.indoors(guest) || g.roomAt(guest.X, guest.Y) == g.roomAt(g.player.X, g.player.Y)) && g.sight(image.Pt(int(guest.X), int(guest.Y)), image.Pt(int(g.player.X), int(g.player.Y)))
}

// roomAt: the Room whose shell holds the world point, -1 if none.
func (g *Game) roomAt(x, y float64) int {
	return slices.IndexFunc(g.hotel.Rooms, func(r world.Room) bool { return image.Pt(int(x), int(y)).In(r.Shell()) })
}

// indoors: the guest is inside a Room.
func (g *Game) indoors(guest *entities.Guest) bool {
	at := image.Pt(int(guest.X), int(guest.Y))
	return slices.ContainsFunc(g.hotel.Rooms, func(r world.Room) bool { return at.In(r.Shell()) })
}

// sight: no wall cell between world points a and b, checked every 4 px; their own cells don't count.
func (g *Game) sight(a, b image.Point) bool {
	n := max(1, int(math.Hypot(float64(b.X-a.X), float64(b.Y-a.Y)))/4)
	for i := 1; i < n; i++ {
		m := g.hotel.Map
		c := m.Cell(a.Add(b.Sub(a).Mul(i).Div(n)))
		if c != m.Cell(a) && c != m.Cell(b) && m.Wall(c.X, c.Y) {
			return false
		}
	}
	return true
}
