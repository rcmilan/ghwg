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
	// the whole Street's box, not just its part on screen, so the weather keeps its place as the camera moves
	x0, y0 := -ox, (g.hotel.Spawn.Y-3*world.TileSize/2)*zoom-oy
	cw, ch := mw*zoom, mh*zoom-(y0+oy)
	if !image.Rect(x0, y0, x0+cw, y0+ch).Overlaps(dst.Bounds()) {
		return
	}
	t := s.n
	switch w {
	case wxRain, wxStorm:
		for i := range 70 * cw / screenW { // streaks falling, as thick as on one screen
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

// fogBlob: a cloud w by h px (world px, drawn at zoom) of overlapping ellipses (cx, cy, rx, ry in 0..1): a
// bright solid core, a checker-dithered shoulder, a sparse dithered rim, and a lumpy outline.
func fogBlob(w, h int, lumps [][4]float64) *ebiten.Image {
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			v := 0.0
			for _, l := range lumps {
				dx, dy := ((float64(x)+.5)/float64(w)-l[0])/l[2], ((float64(y)+.5)/float64(h)-l[1])/l[3]
				v = max(v, 1-math.Sqrt(dx*dx+dy*dy))
			}
			switch {
			case v > .45:
				img.SetNRGBA(x, y, color.NRGBA{244, 247, 252, 255})
			case v > .2 && (x+y)%2 == 0, v > .2 && v <= .45 && y%2 == 0 && x%2 == 0:
				img.SetNRGBA(x, y, color.NRGBA{226, 232, 242, 255})
			case v > 0 && x%2 == 0 && y%2 == 0:
				img.SetNRGBA(x, y, color.NRGBA{214, 222, 236, 255})
			}
		}
	}
	return ebiten.NewImageFromImage(img)
}

// drawFog: fogPuffs banks in the box (screen px), from the ground up to the front wall's foot, drifting east
// slowly and fading toward its left and right edges. The big faint ones drift slower, behind. No per-pixel work.
func (g *Game) drawFog(dst *ebiten.Image, x0, y0, cw, ch int) {
	s := &g.street
	if s.puffs[0] == nil {
		s.puffs = [3]*ebiten.Image{
			fogBlob(44, 22, [][4]float64{{.3, .6, .3, .4}, {.55, .45, .32, .5}, {.78, .62, .24, .36}}),
			fogBlob(34, 18, [][4]float64{{.35, .55, .33, .42}, {.68, .5, .3, .45}}),
			fogBlob(56, 26, [][4]float64{{.22, .62, .22, .36}, {.45, .42, .26, .5}, {.7, .5, .28, .46}, {.88, .66, .14, .3}}),
		}
	}
	for i := range fogPuffs {
		big := i < 4 // behind: twice the size, fainter, slower
		img := s.puffs[i%3]
		sc, al, spd := float64(zoom), .62, 3+i%3
		if big {
			sc, al, spd = 2*zoom, .3, 2
		}
		pw, ph := int(float64(img.Bounds().Dx())*sc), int(float64(img.Bounds().Dy())*sc)
		span := cw + pw
		x := (i*span*5/(fogPuffs*3) + s.n*spd/20) % span
		fx := float64(x) / float64(span)
		a := float32(al * min(1, min(fx, 1-fx)*5))
		y := y0 + (i*ch*5/(fogPuffs*3)+i*29)%max(ch-ph/2, 1) - ph/4 + int(2*zoom*math.Sin(float64(s.n)/90+float64(i)))
		s.op.GeoM.Reset()
		s.op.GeoM.Scale(sc, sc)
		s.op.GeoM.Translate(float64(x0+x-pw), float64(y))
		s.op.ColorScale.Reset()
		s.op.ColorScale.Scale(a, a, a, a)
		dst.DrawImage(img, &s.op)
	}
}
