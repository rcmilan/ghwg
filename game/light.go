package game

import (
	"image"
	"image/color"
	"math"

	"github.com/hajimehoshi/ebiten/v2"
)

type tintKey struct{ h, r, g, b, a float64 }

// nightKey is the darkness at full night: deep blue.
var nightKey = tintKey{0, .04, .06, .22, .72}

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
)

var (
	warmLight = [3]float32{1, .78, .45}
	coolLight = [3]float32{.45, .85, 1}
)

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
		if image.Pt(int(g.player.X), int(g.player.Y)).In(r.Shell()) {
			fill(r.Shell(), 1)
		}
	}
	type light struct {
		at  image.Point
		img *ebiten.Image
		rgb [3]float32
		cut float32
	}
	lights := []light{{g.hotel.Terminal.Min.Add(g.hotel.Terminal.Max).Div(2), g.glow, coolLight, lightCut}}
	for _, p := range g.hotel.Lights {
		lights = append(lights, light{p, g.pool, warmLight, lampCut})
	}
	place := func(l light) *ebiten.DrawImageOptions {
		r := l.img.Bounds().Dx() / 2
		op := &ebiten.DrawImageOptions{Filter: ebiten.FilterLinear}
		op.GeoM.Translate(float64(-r), float64(-r))
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
