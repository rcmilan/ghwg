package game

import (
	"image"
	"image/color"
	"slices"

	"ghosthotel/assets"
	"ghosthotel/entities"
	"ghosthotel/world"

	"github.com/hajimehoshi/ebiten/v2"
)

const (
	fogFrames = tps / 2 // the veil's fade each way, ~0.5 s (D92)
	fogAlpha  = 0.95    // the veil's Night tone at full fog
)

// updateFog fades each Room's veil toward 1 while its door is shut with its whole Party inside, or
// once the Party has settled in, even while it's out (S6 review); toward 0 while the door is open (D92).
func (g *Game) updateFog() {
	if len(g.fog) != len(g.hotel.Rooms) {
		g.fog = make([]float64, len(g.hotel.Rooms))
	}
	for i := range g.fog {
		if g.fogged(i) {
			g.fog[i] = min(g.fog[i]+g.dt/fogFrames, 1)
		} else {
			g.fog[i] = max(g.fog[i]-g.dt/fogFrames, 0)
		}
	}
}

func (g *Game) fogged(room int) bool {
	shell := g.hotel.Rooms[room].Shell()
	if g.hotel.Doors[room].Open() > 0 || g.feet().In(shell) { // never over the Concierge (Live test 5, 2026-11-08)
		return false
	}
	for _, p := range g.parties {
		if p.Occupies(room, Human) {
			return p.settled || !slices.ContainsFunc(p.Guests, func(guest *entities.Guest) bool {
				return !image.Pt(int(guest.X), int(guest.Y)).In(shell)
			})
		}
	}
	return false
}

// fogAt is the fog over world point pt: its Room's, if any. The ghost dimension sees through it (D158).
func (g *Game) fogAt(pt image.Point) float64 {
	if g.ghost {
		return 0
	}
	for i, f := range g.fog {
		if f > 0 && pt.In(g.hotel.Rooms[i].Shell()) {
			return f
		}
	}
	return 0
}

// propFog is the fog over a static tile: a fogged Room's props, and its north wall face (2 rows) with
// its windows. The side lines, the south face and the door stay drawn.
func (g *Game) propFog(p prop) float64 {
	if g.ghost {
		return 0
	}
	if p.room < 0 || p.room >= len(g.fog) || g.fog[p.room] == 0 {
		return 0
	}
	o := g.hotel.Rooms[p.room].Origin
	x, y := p.cx-o.X, p.cy-o.Y
	if p.l == world.LayerProps || y < 2 && !(p.l == world.LayerDecor && (x == 0 || x == world.RoomW-1)) {
		return g.fog[p.room]
	}
	return 0
}

// drawFogFloor veils each fogged Room's floor, between its north and south wall faces (2 rows each);
// the walls, side lines, door and windows draw over it.
func (g *Game) drawFogFloor(screen *ebiten.Image, ox, oy int) {
	const ts = world.TileSize
	for i, f := range g.fog {
		if f == 0 || g.ghost {
			continue
		}
		s := g.hotel.Rooms[i].Shell()
		n := assets.Night
		rect(screen, s.Min.X*zoom-ox, (s.Min.Y+2*ts)*zoom-oy, s.Dx()*zoom, (s.Dy()-4*ts)*zoom,
			color.NRGBA{n.R, n.G, n.B, uint8(255 * fogAlpha * f)})
	}
}
