package game

import (
	"image"
	"math"

	"ghosthotel/world"

	"github.com/hajimehoshi/ebiten/v2"
)

// The hall's light spilling through the Lobby Archway into the Corridor (T172). The hall light is a strip
// as wide as the Archway, spillDepth into the Lobby; a point north of the Hall Wall gets the share of that
// strip it can see past the walls, fading with distance. Many rays across the strip make a soft penumbra.
const (
	spillRes    = 2   // world px per mask pixel
	spillRadius = 176 // world px from the hall light to where the spill is gone (placeholder)
	spillDepth  = 32  // world px the hall light sits below the Hall Wall's foot (placeholder)
	spillRays   = 16
)

// spillLevel scales the spill at clock c: the hall light is the Lobby's.
func spillLevel(c int) float64 { return lobbyLit(c) }

// spillMask is the spill north of the Hall Wall, alpha 0..255, one pixel per spillRes world px; its
// bounds are in those pixels. Built once per map: the walls and doors never move (D45).
func spillMask(h *world.Hotel) *image.Alpha {
	const ts = world.TileSize
	m := h.Map
	wall := make([]bool, m.W*m.H)
	for y := range m.H {
		for x := range m.W {
			wall[y*m.W+x] = m.Wall(x, y)
		}
	}
	for _, d := range h.Doors { // a Room door, shut or not, keeps the hall light out of the Room
		if d.Room < 0 {
			continue
		}
		for y := d.Rect.Min.Y / ts; y < d.Rect.Max.Y/ts; y++ {
			for x := d.Rect.Min.X / ts; x < d.Rect.Max.X/ts; x++ {
				wall[y*m.W+x] = true
			}
		}
	}
	isWall := func(cx, cy int) bool { return cx < 0 || cy < 0 || cx >= m.W || cy >= m.H || wall[cy*m.W+cx] }

	ax, ay := h.Archway.X/ts, h.Archway.Y/ts // a floor cell on the Archway's bottom row
	x0, x1 := ax, ax
	for !isWall(x0-1, ay) {
		x0--
	}
	for !isWall(x1+1, ay) {
		x1++
	}
	top := (ay - 1) * ts // the Hall Wall's top edge; the Lobby's fill covers from here down
	sy := float64((ay+1)*ts + spillDepth)
	cx := float64(x0+x1+1) * ts / 2
	near := sy - float64(top) // full light at the Archway's mouth, to match the lit Lobby

	// visible: a straight line from the hall light at (sx,sy) reaches (px,py). A wall face doesn't hide
	// itself: only its bottom row is a wall cell, so a target on either of its 2 rows sees past its own
	// row and the one below, and faces turned to the Archway light up evenly.
	visible := func(px, py, sx float64) bool {
		tx, ty := int(px)/ts, int(py)/ts
		face := isWall(tx, ty) || isWall(tx, ty+1)
		n := int(math.Hypot(sx-px, sy-py)/spillRes) + 1
		for k := 1; k <= n; k++ {
			f := float64(k) / float64(n)
			c := image.Pt(int(px+(sx-px)*f)/ts, int(py+(sy-py)*f)/ts)
			if isWall(c.X, c.Y) && !(face && (c.Y == ty || c.Y == ty+1)) {
				return false
			}
		}
		return true
	}
	r := image.Rect(int(cx-spillRadius), int(sy-spillRadius), int(cx+spillRadius), top).
		Intersect(image.Rect(0, 0, m.W*ts, m.H*ts))
	mask := image.NewAlpha(image.Rectangle{r.Min.Div(spillRes), r.Max.Div(spillRes)})
	for j := mask.Rect.Min.Y; j < mask.Rect.Max.Y; j++ {
		for i := mask.Rect.Min.X; i < mask.Rect.Max.X; i++ {
			px, py := float64(i*spillRes)+spillRes/2., float64(j*spillRes)+spillRes/2.
			f := min(1, (spillRadius-math.Hypot(px-cx, py-sy))/(spillRadius-near))
			if f <= 0 {
				continue
			}
			seen := 0
			for k := range spillRays {
				if visible(px, py, float64(x0*ts)+(float64(k)+.5)*float64((x1+1-x0)*ts)/spillRays) {
					seen++
				}
			}
			f = f * f * (3 - 2*f) // smoothstep: no visible rim
			mask.Pix[mask.PixOffset(i, j)] = uint8(255 * f * float64(seen) / spillRays)
		}
	}
	return mask
}

// drawSpill cuts the spill out of the darkness layer, as strong as the Lobby is lit.
func (g *Game) drawSpill(ox, oy int) {
	s := spillLevel(g.clock)
	if s == 0 {
		return
	}
	if g.spill == nil {
		g.spill = ebiten.NewImageFromImageWithOptions(spillMask(g.hotel), &ebiten.NewImageFromImageOptions{PreserveBounds: true})
	}
	at := g.spill.Bounds().Min.Mul(spillRes * zoom)
	op := &ebiten.DrawImageOptions{Filter: ebiten.FilterLinear, Blend: ebiten.BlendDestinationOut}
	op.GeoM.Scale(spillRes*zoom, spillRes*zoom)
	op.GeoM.Translate(float64(at.X-ox), float64(at.Y-oy))
	op.ColorScale.ScaleAlpha(float32(s))
	g.dark.DrawImage(g.spill, op)
}
