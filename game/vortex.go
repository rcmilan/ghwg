package game

import (
	"image"
	"math"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"ghosthotel/assets"
	"ghosthotel/entities"
	"ghosthotel/world"

	"github.com/hajimehoshi/ebiten/v2"
)

// 08 T372: the prestige cutscene's vortex. From reelRace to reelFlash everyone but Father P, and every piece of
// furniture whole, lifts off one after another (over liftSpan frames, the nearest to the clock first) from where it
// is on the flipped screen, and tumbles in a spiral round the spinning clock, faster as time speeds up, until the
// clock has swallowed it by the white. Each flyer is drawn once into its own small image (by the world pass's own
// draw code: see take's callers) and that image is turned on the screen.
const (
	liftSpan    = 150 // frames over which the lift-offs are spread: 2.5 s
	vortexTurns = 5   // turns of the orbit over the whole race, at the clock's own curve
	kitchenKey  = "kitchen"
)

// spin: the angle, radians, the vortex has turned by frame t: the race's own spin curve (spinStart to spinEnd, as
// the clock's) integrated.
func spin(t int) float64 {
	k := min(1, max(0, float64(t-reelRace)/(reelFlash-reelRace)))
	ratio := spinEnd / spinStart
	return vortexTurns * 2 * math.Pi * (math.Pow(ratio, k) - 1) / (ratio - 1)
}

// jit: one flyer's own chaos (live test 2026-11-20: "more chaotic", not a line): its orbit speed, tumble rate (either
// way), the fling out before the pull in (px; negative cuts in), the radius wobble and its frequency and phase, and
// how squashed its orbit is.
type jit struct{ speed, tumble, kick, wob, wf, ph, squash float64 }

// jitter: flyer i's chaos, fixed per i (no math/rand: -shot prints repeat).
func jitter(i int) jit {
	h := func(k int) float64 {
		x := uint32(i*7919 + k*104729)
		x = (x ^ x>>16) * 0x45d9f3b
		x = (x ^ x>>16) * 0x45d9f3b
		return float64((x^x>>16)%10000) / 10000
	}
	return jit{speed: .5 + 1.3*h(1), tumble: (2*h(2) - 1) * 4, kick: -40 + 220*h(3), wob: .1 + .3*h(4),
		wf: 2 * math.Pi * (2 + 3*h(5)), ph: 2 * math.Pi * h(6), squash: .5 * h(7)}
}

// spiralAt: where a flyer with chaos j that lifted off at frame start from (x0,y0) is at frame t, round the clock at
// (cx,cy): its screen position, tumble angle (0 at lift-off) and scale. At lift-off it is where it stood; it is
// flung out, wobbles on a squashed orbit at its own speed, and the radius reaches 0 at reelFlash.
func spiralAt(x0, y0, cx, cy float64, start, t int, j jit) (x, y, ang, scale float64) {
	k := min(1, max(0, float64(t-start)/float64(reelFlash-start)))
	u := min(1, float64(t-start)/30) // the first half second: the chaos takes over from where it stood
	r := (math.Hypot(x0-cx, y0-cy) + j.kick*math.Sin(math.Pi*k)) * (1 - k*k) * (1 + j.wob*u*math.Sin(j.wf*k+j.ph))
	d := spin(t) - spin(start)
	a := math.Atan2(y0-cy, x0-cx) + d*j.speed
	return cx + r*math.Cos(a), cy + r*math.Sin(a)*(1-j.squash*u), j.tumble * d, min(1, 4*(1-k))
}

type flyer struct {
	img   *ebiten.Image
	x, y  float64 // where it left, on the screen (flipped)
	start int
	seen  bool
	j     jit
}

type vortex struct {
	fl    map[any]*flyer
	order []*flyer
	reg   bool // the registering pass: nothing is hidden, every flyer in view is made
	w, h  float64
	objs  map[int]*vObject // prop index to its object, once the race starts
}

// take: the world pass met flyer key, whose picture is w x h px centred at buffer px (cx,cy), draw making the
// picture. True once it has lifted off: the world pass leaves it out.
func (r *prestigeReel) take(key any, cx, cy float64, w, h int, draw func(*ebiten.Image)) bool {
	v := &r.vx
	if key == nil || r.skip || r.t < reelRace {
		return false
	}
	if v.reg {
		hw, hh := float64(w)/2, float64(h)/2
		if v.fl[key] == nil && cx+hw > 0 && cx-hw < v.w && cy+hh > 0 && cy-hh < v.h {
			img := ebiten.NewImage(w, h)
			draw(img)
			f := &flyer{img: img, x: cx, y: v.h - cy, j: jitter(len(v.order))}
			v.fl[key] = f
			v.order = append(v.order, f)
		}
		return false
	}
	f := v.fl[key]
	if f == nil || r.t < f.start {
		return false
	}
	if !f.seen { // its place at the frame it left, not at the registering
		f.seen, f.x, f.y = true, cx, v.h-cy
	}
	return true
}

// gone: key has lifted off, so it says nothing.
func (r *prestigeReel) gone(key any) bool {
	f := r.vx.fl[key]
	return f != nil && !r.vx.reg && r.t >= f.start
}

// registerVortex runs the world pass once for the flyers, then orders them by distance from the clock.
func (g *Game) registerVortex(ox, oy int) {
	r, b := g.reel, g.reel.buf.Bounds()
	v := &r.vx
	v.fl, v.reg, v.w, v.h = map[any]*flyer{}, true, float64(b.Dx()), float64(b.Dy())
	g.drawWorld(r.buf, ox, oy)
	v.reg = false
	cx, cy := v.w/2, v.h/2
	near := func(f *flyer) float64 { return math.Hypot(f.x-cx, f.y-cy) * (.4 + 1.2*f.j.ph/(2*math.Pi)) } // nearest first, loosely
	slices.SortStableFunc(v.order, func(a, b *flyer) int { return int(near(a) - near(b)) })
	for i, f := range v.order {
		f.start = reelRace + i*liftSpan/len(v.order)
	}
	r.buf.Fill(assets.Night)
	r.late = r.late[:0]
}

// drawVortex lays every lifted-off flyer on the screen.
func (g *Game) drawVortex(screen *ebiten.Image) {
	r, b := g.reel, screen.Bounds()
	cx, cy := float64(b.Dx())/2, float64(b.Dy())/2
	for _, f := range r.vx.order {
		if r.t < f.start {
			continue
		}
		x, y, ang, s := spiralAt(f.x, f.y, cx, cy, f.start, r.t, f.j)
		if s <= 0 {
			continue
		}
		w, h := f.img.Bounds().Dx(), f.img.Bounds().Dy()
		op := &ebiten.DrawImageOptions{}
		op.GeoM.Translate(-float64(w)/2, -float64(h)/2)
		op.GeoM.Scale(s, -s) // as the flipped world showed it, then tumbling
		op.GeoM.Rotate(ang)
		op.GeoM.Translate(x, y)
		screen.DrawImage(f.img, op)
	}
}

func (r *prestigeReel) freeVortex() {
	for _, f := range r.vx.order {
		f.img.Deallocate()
	}
	r.vx = vortex{}
}

// vObject: one piece of furniture, its tiles drawn as props (indices of g.props) and its box in world px.
type vObject struct {
	tiles []int
	box   image.Rectangle
}

var tileRC = regexp.MustCompile(`^(.*)_r(\d+)(?:c(\d+))?$`)

// flyable: the tile is furniture. Walls, borders, doors, the entrance, windows, curtains and stairs stay.
func flyable(name string) bool {
	switch {
	case strings.HasPrefix(name, "counter_"):
		return true
	case strings.HasPrefix(name, "prop_window_front"), strings.HasPrefix(name, "prop_stairs"):
		return false
	}
	return strings.HasPrefix(name, "prop_")
}

type vTile struct {
	name    string
	l, x, y int
}

// groupTiles: the flyable tiles as objects (lists of indices into ts). A tile named ..._rRcC is the R-th row,
// C-th column of the object with top-left (x-C, y-R); counters, which have no such suffix, glue to their
// neighbours across and down.
func groupTiles(ts []vTile) [][]int {
	type key struct {
		name    string
		l, x, y int
	}
	by, out := map[key]int{}, [][]int(nil)
	counters := map[[3]int]int{} // layer, x, y to tile
	for i, t := range ts {
		m := tileRC.FindStringSubmatch(t.name)
		switch {
		case !flyable(t.name):
		case strings.HasPrefix(t.name, "counter_"):
			counters[[3]int{t.l, t.x, t.y}] = i
		case m != nil:
			r, _ := strconv.Atoi(m[2])
			c, _ := strconv.Atoi(m[3]) // "" (a Mirror's _r0) is column 0
			k := key{m[1], t.l, t.x - c, t.y - r}
			if _, ok := by[k]; !ok {
				by[k] = len(out)
				out = append(out, nil)
			}
			out[by[k]] = append(out[by[k]], i)
		default:
			out = append(out, []int{i})
		}
	}
	seen := map[int]bool{}
	for i, t := range ts { // in tile order: the objects come in a fixed order
		if _, ok := counters[[3]int{t.l, t.x, t.y}]; !ok || seen[i] {
			continue
		}
		var obj []int
		todo := []int{i}
		seen[i] = true
		for len(todo) > 0 {
			j := todo[0]
			todo = todo[1:]
			obj = append(obj, j)
			for _, d := range [...][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}} {
				if n, ok := counters[[3]int{ts[j].l, ts[j].x + d[0], ts[j].y + d[1]}]; ok && !seen[n] {
					seen[n] = true
					todo = append(todo, n)
				}
			}
		}
		out = append(out, obj)
	}
	return out
}

// vortexObjects: g.props' furniture grouped, by prop index; the lobby Mirror, while still covered, stays.
func (g *Game) vortexObjects() map[int]*vObject {
	var ts []vTile
	var at []int
	for i, p := range g.props {
		if !g.covered(p) {
			ts, at = append(ts, vTile{g.ts.Tiles[p.id].Name, p.l, p.cx, p.cy}), append(at, i)
		}
	}
	objs := map[int]*vObject{}
	for _, grp := range groupTiles(ts) {
		o := &vObject{}
		for _, k := range grp {
			i := at[k]
			p := g.props[i]
			or := g.hotel.Map.Origin(p.cx, p.cy)
			o.tiles = append(o.tiles, i)
			o.box = o.box.Union(image.Rectangle{Min: or, Max: or.Add(g.tiles[p.id].Bounds().Size())})
			objs[i] = o
		}
	}
	return objs
}

// vortexProp: the world pass met props[i]: true if its object has lifted off.
func (g *Game) vortexProp(i, ox, oy int) bool {
	v := &g.reel.vx
	if v.objs == nil {
		v.objs = g.vortexObjects()
	}
	o := v.objs[i]
	if o == nil {
		return false
	}
	return g.reel.take(o, float64((o.box.Min.X+o.box.Max.X)*zoom/2-ox), float64((o.box.Min.Y+o.box.Max.Y)*zoom/2-oy), o.box.Dx()*zoom, o.box.Dy()*zoom, func(dst *ebiten.Image) {
		for _, j := range o.tiles {
			p := g.props[j]
			or := g.hotel.Map.Origin(p.cx, p.cy).Sub(o.box.Min)
			op := &ebiten.DrawImageOptions{}
			op.GeoM.Scale(zoom, zoom)
			op.GeoM.Translate(float64(or.X*zoom), float64(or.Y*zoom))
			dst.DrawImage(g.tiles[p.id], op)
		}
	})
}

// vortexKitchen: the kitchen's props are one object; true once it has lifted off.
func (g *Game) vortexKitchen(ox, oy int) bool {
	k := g.hotel.Stock
	k.Min.Y -= world.TileSize
	return g.reel.take(kitchenKey, float64((k.Min.X+k.Max.X)*zoom/2-ox), float64((k.Min.Y+k.Max.Y)*zoom/2-oy), k.Dx()*zoom, k.Dy()*zoom, func(dst *ebiten.Image) {
		g.drawKitchen(dst, k.Min.X*zoom, k.Min.Y*zoom)
	})
}

// vortexActor: the world pass met an actor (a character, or a plant standing on (x,y), world px): true once it has
// lifted off. draw puts it in its own image at the foot point (x,y) given.
func (g *Game) vortexActor(key any, name string, x, y float64, pot bool, ox, oy int, draw func(dst *ebiten.Image, x, y int)) bool {
	if key == nil {
		return false
	}
	w, h := charW*zoom, charH*zoom
	if pot {
		img := g.sprite(name)
		w, h = img.Bounds().Dx()*zoom, img.Bounds().Dy()*zoom
	}
	fx, fy := screenPx(x)-ox, screenPx(y)-oy      // as the world pass's at()
	cx, cy, py := float64(fx), float64(fy-h/2), h // a plant's foot is its bottom middle
	if !pot {                                     // a character's foot box center is FootH/2 above its sprite's bottom
		cy, py = float64(fy+(entities.FootH/2-charH/2)*zoom), (charH-entities.FootH/2)*zoom
	}
	return g.reel.take(key, cx, cy, w, h, func(dst *ebiten.Image) { draw(dst, w/2, py) })
}
