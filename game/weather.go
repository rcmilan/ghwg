package game

import (
	"image"
	"image/color"
	"math"

	"ghosthotel/world"

	"github.com/hajimehoshi/ebiten/v2"
)

// 08 W5-STREET: ambient weather (visual only) and the thunderstorm.
const (
	wxClear = iota
	wxRain
	wxFog
	wxWind
	wxStorm // not rolled: the Storm jar's, or the dev flag's
)

// Odds of each day's weather, of 100 (placeholders): the rest is clear. Dims: how much of the day it takes, 0..1.
const (
	rainPct  = 20
	fogPct   = 12
	windPct  = 12
	rainDim  = 0.22
	stormDim = 0.38
	fogDim   = 0.10
	flashGap = 90 // frames between lightning flashes: this plus up to flashVar
	flashVar = 300
)

var rainDark = [3]float64{.16, .20, .34}

// weatherOn: the weather of a Day, rolled from the Day alone (a hash), so it moves none of the sim's streams.
func (g *Game) weatherOn(d int) int {
	x := uint64(d)*0x9e3779b97f4a7c15 + 0x57e4
	x = (x ^ x>>30) * 0xbf58476d1ce4e5b9
	x = (x ^ x>>27) * 0x94d049bb133111eb
	switch r := int((x ^ x>>31) % 100); {
	case r < rainPct:
		return wxRain
	case r < rainPct+fogPct:
		return wxFog
	case r < rainPct+fogPct+windPct:
		return wxWind
	}
	return wxClear
}

// weather: what it is doing now.
func (g *Game) weather() int {
	switch {
	case g.street.force != 0:
		return g.street.force
	case g.stormOn():
		return wxStorm
	}
	return g.weatherOn(day(g.clock))
}

// summonStorm: tonight's thunderstorm (the Storm jar): lightning, and sightings pay double until dawn.
func (g *Game) summonStorm() {
	u := g.clock - g.clock%dayFrames + runStart // the next 08:00
	if u <= g.clock {
		u += dayFrames
	}
	g.street.stormUntil = u
}

// stormOn: a thunderstorm is on now: sightings pay double (shadow.go).
func (g *Game) stormOn() bool {
	u := g.street.stormUntil
	return u > g.clock && u-g.clock <= dayFrames // a storm of an earlier Run does not carry over
}

// updateWeather: lightning while it storms.
func (g *Game) updateWeather() {
	s := &g.street
	s.flash = max(0, s.flash-0.07)
	if g.weather() == wxStorm && s.n >= s.nextFlash {
		s.flash, s.nextFlash = 1, s.n+flashGap+s.rng.IntN(flashVar)
	}
}

// weatherTint: the darkness layer's color and alpha under the weather: a gray-blue veil, a lightning flash taking
// the darkness away for a moment.
func (g *Game) weatherTint(t [4]float64) [4]float64 {
	dim := 0.0
	switch g.weather() {
	case wxRain:
		dim = rainDim
	case wxStorm:
		dim = stormDim
	case wxFog:
		dim = fogDim
	}
	if dim > 0 {
		a := t[3] + (1-t[3])*dim
		for i := range 3 {
			t[i] = (t[i]*t[3] + rainDark[i]*(1-t[3])*dim) / a
		}
		t[3] = a
	}
	if f := g.street.flash; f > 0 {
		for i := range 3 {
			t[i] += (.85 - t[i]) * f
		}
		t[3] *= 1 - .85*f
	}
	return t
}

// stroke draws a w×h screen px rectangle of color (r,g,b) and opacity a at (x,y), no allocation.
func (s *streetState) stroke(dst *ebiten.Image, x, y, w, h float64, r, g, b, a float32) {
	if s.px == nil {
		img := image.NewNRGBA(image.Rect(0, 0, 1, 1))
		img.SetNRGBA(0, 0, color.NRGBA{255, 255, 255, 255})
		s.px = ebiten.NewImageFromImage(img)
	}
	s.op.GeoM.Reset()
	s.op.GeoM.Scale(w, h)
	s.op.GeoM.Translate(x, y)
	s.op.ColorScale.Reset()
	s.op.ColorScale.Scale(r*a, g*a, b*a, a)
	dst.DrawImage(s.px, &s.op)
}

// drawWeather: rain on the Street and the windows, fog over the Street, wind streaks; the view's top left is (ox,oy).
func (g *Game) drawWeather(dst *ebiten.Image, ox, oy int) {
	s := &g.street
	w := g.weather()
	if w == wxClear {
		return
	}
	mw, mh := g.mapSize()
	b := dst.Bounds()
	x0, x1 := max(-ox, b.Min.X), min(mw*zoom-ox, b.Max.X)
	y0, y1 := max((g.hotel.Spawn.Y-3*world.TileSize/2)*zoom-oy, b.Min.Y), min(mh*zoom-oy, b.Max.Y)
	if x1 <= x0 || y1 <= y0 {
		return
	}
	cw, ch := x1-x0, y1-y0
	t := s.n
	switch w {
	case wxRain, wxStorm:
		for i := range 70 { // streaks falling
			x := x0 + (i*61+t*2)%cw
			y := y0 + (i*127+t*9)%ch
			s.stroke(dst, float64(x), float64(y), 2, 9, .75, .85, 1, .55)
		}
		for _, p := range g.hotel.Windows { // and down the glass above each window's light
			for k := range 3 {
				x := p.X*zoom - ox + (k-1)*7*zoom
				y := (p.Y-26)*zoom - oy + (t*3+k*17)%(18*zoom)
				s.stroke(dst, float64(x), float64(y), 2, 7, .75, .85, 1, .5)
			}
		}
	case wxFog:
		g.drawFog(dst, x0, y0, cw, ch)
	case wxWind:
		for i := range 14 { // streaks racing east
			x := x0 + (i*97+t*7)%(cw+40) - 40
			y := y0 + (i*53)%ch
			s.stroke(dst, float64(x), float64(y), 22, 2, .9, .9, .9, .3)
		}
	}
}

// DevWeather: this weather every day: rain, fog, wind, or storm (the thunderstorm, on from the start).
func (g *Game) DevWeather(kind string) {
	switch kind {
	case "rain":
		g.street.force = wxRain
	case "fog":
		g.street.force = wxFog
	case "wind":
		g.street.force = wxWind
	case "storm":
		g.street.force = wxStorm
		g.summonStorm()
	}
}

// fogPuffs: soft puffs drifting along the Street, each of one of a few pre-rendered pixel blobs.
const fogPuffs = 12

// fogBlob: a puff w by h px (world px, drawn at zoom): a solid core, then a checker-dithered ring, then nothing.
func fogBlob(w, h int) *ebiten.Image {
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			dx, dy := (float64(x)+.5)/float64(w)*2-1, (float64(y)+.5)/float64(h)*2-1
			d := dx*dx + dy*dy
			if d < .45 || (d < 1 && (x+y)%2 == 0 && d < .75) || (d < 1 && x%2 == 0 && y%2 == 0) {
				img.SetNRGBA(x, y, color.NRGBA{228, 234, 240, 255})
			}
		}
	}
	return ebiten.NewImageFromImage(img)
}

// drawFog: fogPuffs puffs in the box (screen px), drifting east slowly at ground level, fading toward its left
// and right edges. No per-pixel work: a few images made once.
func (g *Game) drawFog(dst *ebiten.Image, x0, y0, cw, ch int) {
	s := &g.street
	if s.puffs[0] == nil {
		s.puffs = [3]*ebiten.Image{fogBlob(34, 12), fogBlob(24, 9), fogBlob(44, 14)}
	}
	for i := range fogPuffs {
		img := s.puffs[i%3]
		pw, ph := img.Bounds().Dx()*zoom, img.Bounds().Dy()*zoom
		span := cw + pw
		x := (i*span/fogPuffs + s.n*(3+i%3)/20) % span // 0.15-0.25 px a frame, each its own pace
		fx := float64(x) / float64(span)
		a := float32(.32 * min(1, min(fx, 1-fx)*5))
		y := y0 + ch/3 + (i*53)%(ch*2/3-ph/2+1) + int(2*zoom*math.Sin(float64(s.n)/90+float64(i)))
		s.op.GeoM.Reset()
		s.op.GeoM.Scale(zoom, zoom)
		s.op.GeoM.Translate(float64(x0+x-pw), float64(y))
		s.op.ColorScale.Reset()
		s.op.ColorScale.Scale(a, a, a, a)
		dst.DrawImage(img, &s.op)
	}
}
