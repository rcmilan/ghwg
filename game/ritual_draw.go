package game

import (
	"image"
	"math"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"ghosthotel/world"
)

// The Fathers' rituals, drawn with shapes and the glow only (B1 live test, 2026-11-15). Father K is the quietest:
// he bows over a faint ring on the floor, a few soft motes drift up, one gentle pulse lands the effect. Father M
// has presence: he hovers, traces a sigil on the floor that fills in, columns of light stand around it, motes
// converge on him, and the burst sweeps the whole Room in a ring.
const (
	sigilFrames = 90 // frames Father M takes to trace his circle
)

var (
	kTint = [3]float32{.75, .88, 1} // K: a pale moon-blue
	mTint = [3]float32{1, .82, .5}  // M: gold
	mEdge = [3]float32{.7, .55, 1}  // M's sigil: violet
)

// lift: how high the Priest hovers in his chant, world px: K barely stirs, M floats up on his chant.
func (pr *priest) lift() float64 {
	if pr.phase != rChant {
		return 0
	}
	if pr.strong {
		return 5 * math.Min(1, pr.t/60) * (.8 + .2*math.Sin(pr.t/20))
	}
	return .8 * math.Abs(math.Sin(math.Pi*pr.t/(3*chantHop))) // a slow bow
}

// ritualLights: the Room's ritual glows at night too (light pass); nil when none is under way.
func (g *Game) ritualLights() (ls []light) {
	for _, pr := range g.priests {
		if pr.phase != rChant && pr.phase != rBurst {
			continue
		}
		cut, rgb := float32(.35), kTint
		if pr.strong {
			cut, rgb = .6, mTint
		}
		if pr.phase == rBurst {
			cut = min(.95, cut+.35*float32(math.Sin(math.Pi*pr.t/burstFrames)))
		}
		ls = append(ls, light{at: image.Pt(int(pr.X), int(pr.Y)), img: g.glow, rgb: rgb, cut: cut})
	}
	return
}

// strokeP strokes p additively in tint c at alpha a, w px wide.
func strokeP(s *ebiten.Image, p *vector.Path, w float32, c [3]float32, a float64) {
	d := &vector.DrawPathOptions{AntiAlias: true, Blend: ebiten.BlendLighter}
	d.ColorScale.Scale(c[0], c[1], c[2], 1)
	d.ColorScale.ScaleAlpha(float32(math.Max(0, math.Min(1, a))))
	vector.StrokePath(s, p, &vector.StrokeOptions{Width: w}, d)
}

// drawRing strokes the circle of radius r (screen px) on the floor at x,y, from the top round by frac of a turn.
func drawRing(s *ebiten.Image, x, y, r, frac float64, w float32, c [3]float32, a float64) {
	if frac <= 0 || a <= 0 {
		return
	}
	var p vector.Path
	n := max(2, int(48*math.Min(1, frac)))
	for i := 0; i <= n; i++ {
		t := 2*math.Pi*math.Min(1, frac)*float64(i)/float64(n) - math.Pi/2
		px, py := float32(x+r*math.Cos(t)), float32(y+r*math.Sin(t))
		if i == 0 {
			p.MoveTo(px, py)
		} else {
			p.LineTo(px, py)
		}
	}
	strokeP(s, &p, w*zoom/3, c, a)
}

// glowAt: the soft light sprite, additive, d screen px across.
func (g *Game) glowAt(s *ebiten.Image, x, y, d float64, c [3]float32, a float64) {
	k := d / float64(g.glow.Bounds().Dx())
	op := &ebiten.DrawImageOptions{Filter: ebiten.FilterLinear, Blend: ebiten.BlendLighter}
	op.GeoM.Translate(-float64(g.glow.Bounds().Dx())/2, -float64(g.glow.Bounds().Dy())/2)
	op.GeoM.Scale(k, k)
	op.GeoM.Translate(x, y)
	op.ColorScale.Scale(c[0], c[1], c[2], 1)
	op.ColorScale.ScaleAlpha(float32(math.Max(0, math.Min(1, a))))
	s.DrawImage(g.glow, op)
}

// drawRituals: each Priest's ritual by phase and Father. The floor pass (floor true: pools, rings, the sigil) goes
// under the actors, the camera looking straight down, so the circles are round and the Priest stands over them
// (08 B1 live test 4); the rest (motes, columns, flashes) over them.
func (g *Game) drawRituals(s *ebiten.Image, ox, oy int, floor bool) {
	for _, pr := range g.priests {
		x, y := pr.X*zoom-float64(ox), pr.Y*zoom-float64(oy)
		if pr.strong {
			g.drawM(s, pr, x, y, floor)
		} else {
			g.drawK(s, pr, x, y, floor)
		}
	}
}

func (g *Game) drawK(s *ebiten.Image, pr *priest, x, y float64, floor bool) {
	const r = 14 * zoom
	switch {
	case pr.phase == rChant && floor:
		f := math.Min(1, pr.t/60)
		breath := .5 + .5*math.Sin(pr.t/45)
		g.glowAt(s, x, y, 1.6*r*(1+.06*breath), kTint, .3*f)      // a faint pool under him
		drawRing(s, x, y, r, 1, 1.5, kTint, .75*f*(.6+.4*breath)) // the ring, breathing
	case pr.phase == rChant:
		f := math.Min(1, pr.t/60)
		for k := range 3 { // three motes, no more
			u := math.Mod(pr.t/180+float64(k)/3, 1)
			g.glowAt(s, x+math.Sin(float64(k)*2.4+u*2)*7*zoom, y-(4+22*u)*zoom, 7*zoom, kTint, .8*math.Sin(math.Pi*u)*f)
		}
	case pr.phase == rBurst && floor:
		f := pr.t / burstFrames
		drawRing(s, x, y, r*(1+.6*f), 1, 1.5, kTint, .5*(1-f))
	case pr.phase == rBurst:
		f := pr.t / burstFrames
		g.glowAt(s, x, y-6*zoom, 3*world.TileSize*zoom*(.6+.4*f), kTint, .55*math.Sin(math.Pi*f)) // one soft pulse
	}
}

func (g *Game) drawM(s *ebiten.Image, pr *priest, x, y float64, floor bool) {
	const r = 30 * zoom
	switch {
	case pr.phase == rChant && floor:
		f := math.Min(1, pr.t/sigilFrames)
		g.glowAt(s, x, y, 2.4*r, mEdge, .2*f)
		drawRing(s, x, y, r, f, 2, mEdge, .85)      // the circle traces itself round
		drawRing(s, x, y, r*.62, f, 1.5, mTint, .7) // and the inner one the other way
		if f >= 1 {                                 // then a triangle pair joins them: six points, twelve rays
			g.drawStar(s, x, y, r*.62, pr.t/400)
		}
		for k := range 4 { // where the columns stand
			a := float64(k)*math.Pi/2 + math.Pi/4 + pr.t/300
			g.glowAt(s, x+r*math.Cos(a), y+r*math.Sin(a), 10*zoom, mTint, .5*f)
		}
	case pr.phase == rChant:
		f := math.Min(1, pr.t/sigilFrames)
		for k := range 4 { // light columns stand on the circle
			a := float64(k)*math.Pi/2 + math.Pi/4 + pr.t/300
			cx, cy := x+r*math.Cos(a), y+r*math.Sin(a)
			h := (22 + 6*math.Sin(pr.t/20+float64(k))) * zoom * f
			var col vector.Path
			col.MoveTo(float32(cx), float32(cy))
			col.LineTo(float32(cx), float32(cy-h))
			strokeP(s, &col, 3, mTint, .35*f)
		}
		for k := range 8 { // motes swirl in to him
			u := math.Mod(pr.t/90+float64(k)/8, 1)
			a := float64(k)*.785 + u*3
			rr := r * (1 - u)
			g.glowAt(s, x+rr*math.Cos(a), y-u*10*zoom+rr*math.Sin(a), 5*zoom, mTint, .8*math.Sin(math.Pi*u)*f)
		}
	case pr.phase == rBurst && floor:
		f := pr.t / burstFrames
		drawRing(s, x, y, r, 1, 3, mEdge, 1-f)
		for k, c := range [...][3]float32{mTint, mEdge} { // two rings sweep out past the walls, the gold first
			e := math.Max(0, f*1.25-.25*float64(k))
			drawRing(s, x, y, r*(1+7*e), 1, 8-4*float32(e), c, 1.2*(1-e))
		}
	case pr.phase == rBurst:
		f := pr.t / burstFrames
		g.glowAt(s, x, y-8*zoom, 3*world.TileSize*zoom*(.5+f)*3, mTint, .65*math.Sin(math.Pi*f)) // the flash, a Room wide
		g.glowAt(s, x, y-8*zoom, 3*world.TileSize*zoom*(.5+f), [3]float32{1, 1, 1}, .5*(1-f))
	}
}

// drawStar: a hexagram, two triangles, in violet.
func (g *Game) drawStar(s *ebiten.Image, x, y, r, rot float64) {
	for t := range 2 {
		var p vector.Path
		for i := 0; i <= 3; i++ {
			a := rot + math.Pi/6*float64(3*t) + 2*math.Pi*float64(i)/3 - math.Pi/2
			px, py := float32(x+r*math.Cos(a)), float32(y+r*math.Sin(a))
			if i == 0 {
				p.MoveTo(px, py)
			} else {
				p.LineTo(px, py)
			}
		}
		strokeP(s, &p, 1.2, mEdge, .7)
	}
}
