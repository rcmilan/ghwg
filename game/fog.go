package game

import (
	"image"
	"image/color"
	"slices"

	"ghosthotel/assets"
	"ghosthotel/entities"
	"ghosthotel/world"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/colorm"
)

const (
	fogFrames = tps / 2 // the veil's fade each way, ~0.5 s (D92)
	fogAlpha  = 0.95    // the veil's Night tone at full fog
)

// updateFog fades each Room's veil toward 1 while its whole Party is inside with the door shut, and
// toward 0 from the moment the door opens (D92).
func (g *Game) updateFog() {
	if len(g.fog) != len(g.hotel.Rooms) {
		g.fog = make([]float64, len(g.hotel.Rooms))
	}
	for i := range g.fog {
		if g.fogged(i) {
			g.fog[i] = min(g.fog[i]+1.0/fogFrames, 1)
		} else {
			g.fog[i] = max(g.fog[i]-1.0/fogFrames, 0)
		}
	}
}

func (g *Game) fogged(room int) bool {
	if g.hotel.Doors[room].Open() > 0 {
		return false
	}
	shell := g.hotel.Rooms[room].Shell()
	for _, p := range g.parties {
		if p.Occupies(room) {
			return p.state != out && !slices.ContainsFunc(p.Guests, func(guest *entities.Guest) bool {
				return !image.Pt(int(guest.X), int(guest.Y)).In(shell)
			})
		}
	}
	return false
}

// fogAt is the fog over world point pt: its Room's, if any.
func (g *Game) fogAt(pt image.Point) float64 {
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
	c := image.Pt(p.cx, p.cy)
	for i, f := range g.fog {
		o := g.hotel.Rooms[i].Origin
		if f == 0 || !c.In(image.Rect(o.X, o.Y, o.X+world.RoomW, o.Y+world.RoomH)) {
			continue
		}
		x, y := c.X-o.X, c.Y-o.Y
		if p.l == world.LayerProps || y < 2 && !(p.l == world.LayerDecor && (x == 0 || x == world.RoomW-1)) {
			return f
		}
		return 0
	}
	return 0
}

// drawFogFloor veils each fogged Room's floor, between its north and south wall faces (2 rows each);
// the walls, side lines, door and windows draw over it.
func (g *Game) drawFogFloor(screen *ebiten.Image, ox, oy int) {
	const ts = world.TileSize
	for i, f := range g.fog {
		if f == 0 {
			continue
		}
		s := g.hotel.Rooms[i].Shell()
		n := assets.Night
		rect(screen, s.Min.X*zoom-ox, (s.Min.Y+2*ts)*zoom-oy, s.Dx()*zoom, (s.Dy()-4*ts)*zoom,
			color.NRGBA{n.R, n.G, n.B, uint8(255 * fogAlpha * f)})
	}
}

// drawFogged draws src like dst.DrawImage, tinted toward the Night tone by fog: a fogged Room's props
// and north wall.
func drawFogged(dst, src *ebiten.Image, op *ebiten.DrawImageOptions, fog float64) {
	if fog == 0 {
		dst.DrawImage(src, op)
		return
	}
	a, n := fogAlpha*fog, assets.Night
	var cm colorm.ColorM
	cm.Scale(1-a, 1-a, 1-a, 1)
	cm.Translate(a*float64(n.R)/255, a*float64(n.G)/255, a*float64(n.B)/255, 0)
	colorm.DrawImage(dst, src, cm, &colorm.DrawImageOptions{GeoM: op.GeoM})
}
