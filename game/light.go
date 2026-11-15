package game

import (
	"image"
	"image/color"
	"math"

	"ghosthotel/entities"
	"ghosthotel/world"

	"github.com/hajimehoshi/ebiten/v2"
)

type tintKey struct{ h, r, g, b, a float64 }

// nightKey is the darkness at full night: deep blue.
var nightKey = tintKey{0, .04, .06, .22, .8}

// night is nightKey at hour h.
func night(h float64) tintKey { k := nightKey; k.h = h; return k }

// tintKeys are the darkness layer's color and alpha through the day (D94), by hour; tint interpolates
// linearly between keys, wrapping at midnight. The color changes only while the alpha is 0 or as it
// fades. Keys are at most a game hour apart per unit of change, so nothing jumps. T169, user's pick:
// dawn goes from the blue hour to pale gold; dusk from copper to plum.
var tintKeys = []tintKey{night(0), night(4.5),
	{5.5, .16, .24, .52, .58}, {6.5, .98, .84, .52, .24}, {7.5, .98, .84, .52, 0},
	{17, .95, .55, .28, 0}, {18.5, .90, .48, .24, .30}, {19.75, .36, .16, .40, .50}, {21, .08, .07, .26, .68},
	night(22), night(24)}

// tint is the darkness layer's straight color and alpha at clock c: r, g, b, a in 0..1.
func tint(c int) [4]float64 {
	h := float64(c%dayFrames) / hourFrames
	for i := 1; i < len(tintKeys); i++ {
		k0, k1 := tintKeys[i-1], tintKeys[i]
		if h <= k1.h {
			f := (h - k0.h) / (k1.h - k0.h)
			mix := func(a, b float64) float64 { return a + (b-a)*f }
			return [4]float64{mix(k0.r, k1.r), mix(k0.g, k1.g), mix(k0.b, k1.b), mix(k0.a, k1.a)}
		}
	}
	panic("unreachable")
}

// The Lobby's lights (T170), in minutes of the day (placeholders): on through the evening, fading off
// until lobbyOff (the end of a regular worker's shift), dark until daylight, then fading on from lobbyOn.
const (
	lobbyOff  = 60
	lobbyOn   = 7 * 60
	lobbyFade = 15
)

// lobbyLit is how lit the Lobby is at clock c, 0..1.
func lobbyLit(c int) float64 {
	m := float64(c%dayFrames) / minuteFrames
	return min(1, max(0, (lobbyOff-m)/lobbyFade, (m-lobbyOn)/lobbyFade))
}

const (
	lightRadius = 56   // world px: a ceiling light's pool
	glowRadius  = 24   // world px: the Terminal's glow
	lightCut    = 0.85 // how much of the darkness the Terminal's glow removes at its center
	lampCut     = 0.55 // and a Corridor lamp's: dim but readable (T171, placeholder)
	lightGlow   = 0.18 // a light's own tint, added at its center at full night
	windowCut   = 0.4  // a window's light on the floor below it (F0 Lighting, placeholder)
	lanternCut  = 0.6  // a lantern's beam
	lanternR    = 48   // world px: how far it reaches
	lanternHalf = 0.6  // radians: half its beam's width
)

var (
	warmLight  = [3]float32{1, .78, .45}
	coolLight  = [3]float32{.45, .85, 1}
	moonLight  = [3]float32{.6, .7, 1}
	lanternImg = lightCone(lanternR)
)

// lightCone is lightPool's beam: pointing right from its center, lanternHalf to each side.
func lightCone(radius int) *ebiten.Image {
	img := image.NewNRGBA(image.Rect(0, 0, 2*radius, 2*radius))
	for y := range 2 * radius {
		for x := range 2 * radius {
			dx, dy := float64(x-radius)+.5, float64(y-radius)+.5
			d, side := math.Hypot(dx, dy)/float64(radius), 1-math.Abs(math.Atan2(dy, dx))/lanternHalf
			if d < 1 && side > 0 {
				img.SetNRGBA(x, y, color.NRGBA{255, 255, 255, uint8(255 * (1 - d) * min(1, 2*side))})
			}
		}
	}
	return ebiten.NewImageFromImage(img)
}

type light struct {
	at   image.Point
	img  *ebiten.Image
	rgb  [3]float32
	cut  float32
	turn float64 // radians: a lantern's facing (its beam points right unturned)
}

// lights: the Terminal's glow, the Corridor lamps, the windows, the kitchen's own lights, and a lantern ahead
// of the Concierge and each staff member (F0 Lighting).
func (g *Game) lights() []light {
	h := g.hotel
	ls := []light{{at: h.Terminal.Min.Add(h.Terminal.Max).Div(2), img: g.glow, rgb: coolLight, cut: lightCut}}
	for _, p := range h.Lights {
		ls = append(ls, light{at: p, img: g.pool, rgb: warmLight, cut: lampCut})
	}
	for _, p := range h.Windows {
		l := light{at: p, img: g.pool, rgb: moonLight, cut: windowCut}
		if p.Y >= h.HallY { // the front's: the hall's light, out on the street
			l.rgb = warmLight
		}
		ls = append(ls, l)
	}
	k := h.Kitchen
	for _, f := range [...]int{1, 3} { // over its two halves, just in front of the stock: on after the Lobby's go off
		ls = append(ls, light{at: image.Pt(k.Min.X+k.Dx()*f/4, h.Stock.Max.Y+world.TileSize), img: g.pool, rgb: warmLight, cut: lampCut})
	}
	lantern := func(x, y float64, a entities.Anim) {
		ls = append(ls, light{at: image.Pt(int(x), int(y)), img: lanternImg, rgb: warmLight, cut: lanternCut, turn: -float64(a.Facing) * math.Pi / 2})
	}
	lantern(g.player.X, g.player.Y, g.player.Anim)
	for _, k := range g.keepers {
		lantern(k.X, k.Y, k.Anim)
	}
	for _, p := range g.priests {
		lantern(p.X, p.Y, p.Anim)
	}
	for _, c := range g.couriers {
		if c.out {
			lantern(c.X, c.Y, c.Anim)
		}
	}
	return ls
}

// lightPool is a soft radial sprite, alpha 1 at the center fading to 0 at the rim (D94).
func lightPool(radius int) *ebiten.Image {
	img := image.NewNRGBA(image.Rect(0, 0, 2*radius, 2*radius))
	for y := range 2 * radius {
		for x := range 2 * radius {
			d := math.Hypot(float64(x-radius)+.5, float64(y-radius)+.5) / float64(radius)
			if d < 1 {
				img.SetNRGBA(x, y, color.NRGBA{255, 255, 255, uint8(255 * (1 - d) * (1 - d))})
			}
		}
	}
	return ebiten.NewImageFromImage(img)
}

// drawDarkness lays the hour's darkness over dst (the world drawn with top-left ox,oy screen px), with
// each light's pool cut out of it and a faint glow of its color added (D94, D95).
func (g *Game) drawDarkness(dst *ebiten.Image, ox, oy int) {
	t := tint(g.clock)
	if t[3] == 0 {
		return
	}
	b := dst.Bounds()
	if g.dark == nil || g.dark.Bounds().Size() != b.Size() {
		g.dark = ebiten.NewImage(b.Dx(), b.Dy())
	}
	shade := func(lit float64) color.NRGBA { // the darkness with lit (0..1) of it taken away
		return color.NRGBA{uint8(255 * t[0]), uint8(255 * t[1]), uint8(255 * t[2]), uint8(255 * t[3] * (1 - lit))}
	}
	g.dark.Fill(shade(0))
	fill := func(r image.Rectangle, lit float64) { // a lit area: the Lobby, the Room the Concierge is in
		r = image.Rectangle{r.Min.Mul(zoom), r.Max.Mul(zoom)}.Sub(image.Pt(ox, oy)).Add(b.Min)
		g.dark.SubImage(r).(*ebiten.Image).Fill(shade(lit))
	}
	fill(g.hotel.Lobby, lobbyLit(g.clock))
	g.drawSpill(ox, oy)
	for _, r := range g.hotel.Rooms {
		if sh := r.Shell(); image.Pt(int(g.player.X), int(g.player.Y)).In(sh) {
			fill(image.Rect(sh.Min.X, sh.Min.Y, sh.Max.X, sh.Max.Y-2*world.TileSize), 1) // not its front wall, seen from the Corridor (Live test 4, 2026-11-08)
		}
	}
	lights := g.lights()
	place := func(l light) *ebiten.DrawImageOptions {
		r := l.img.Bounds().Dx() / 2
		op := &ebiten.DrawImageOptions{Filter: ebiten.FilterLinear}
		op.GeoM.Translate(float64(-r), float64(-r))
		op.GeoM.Rotate(l.turn)
		op.GeoM.Scale(zoom, zoom)
		op.GeoM.Translate(float64(l.at.X*zoom-ox), float64(l.at.Y*zoom-oy))
		return op
	}
	for _, l := range lights { // the pools: holes in the darkness
		op := place(l)
		op.ColorScale.ScaleAlpha(l.cut)
		op.Blend = ebiten.BlendDestinationOut
		g.dark.DrawImage(l.img, op)
	}
	dst.DrawImage(g.dark, nil)
	for _, l := range lights { // and their own color, stronger the darker the hour
		op := place(l)
		k := float32(lightGlow * t[3])
		op.ColorScale.Scale(l.rgb[0]*k, l.rgb[1]*k, l.rgb[2]*k, 0)
		op.Blend = ebiten.BlendLighter
		dst.DrawImage(l.img, op)
	}
}
