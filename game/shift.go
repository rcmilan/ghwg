package game

import (
	"ghosthotel/world"

	"github.com/hajimehoshi/ebiten/v2"
)

// Mirror transitions (D170): SPACE at the Mirror plays one of nine effects, picked at random, from the
// real picture to the ghost one. The world keeps running; the dimension switches halfway.
const flipFrames = 48 // 0.8 s, every effect

var transitions = [...]string{"shatter", "ripple", "crt", "glitch", "wave", "iris", "dissolve", "tiles", "card"}

// shift: a transition under way.
type shift struct {
	effect  int
	start   int // g.tick it began
	toGhost bool
	seed    float32 // varies the effect: shard and slice patterns, the wave's axis
}

// startShift begins a transition to the other dimension; the talk's random source picks it, so the sim's
// stays untouched.
func (g *Game) startShift() {
	g.shifting = &shift{effect: g.chat.IntN(len(transitions)), start: g.tick, toGhost: !g.ghost, seed: g.chat.Float32()}
}

// updateShift switches the dimension halfway through and ends the transition after flipFrames.
func (g *Game) updateShift() {
	s := g.shifting
	if s == nil {
		return
	}
	e := g.tick - s.start
	if e >= flipFrames/2 && g.ghost != s.toGhost {
		g.flip()
	}
	if e >= flipFrames {
		g.shifting = nil
	}
}

// ghostness: 0 shows the real picture, 1 the ghost one. Real → ghost runs it forward; ghost → real
// backward (the shards fly back).
func (s *shift) ghostness(tick int) float32 {
	p := min(1, float32(tick-s.start)/flipFrames)
	if !s.toGhost {
		p = 1 - p
	}
	return p
}

// drawShift draws the whole screen in both dimensions and blends them through the transition.
func (g *Game) drawShift(screen *ebiten.Image) {
	w, h := screen.Bounds().Dx(), screen.Bounds().Dy()
	if g.fx[0] == nil {
		g.fx = [2]*ebiten.Image{ebiten.NewImage(w, h), ebiten.NewImage(w, h)}
	}
	if g.transition == nil {
		var err error
		if g.transition, err = ebiten.NewShader([]byte(transitionSrc)); err != nil {
			panic(err) // a compile error in a constant: a bug, caught on the first flip
		}
	}
	was := g.ghost
	for i, img := range g.fx {
		img.Clear()
		g.ghost = i == 1
		g.drawFrame(img)
	}
	g.ghost = was
	mw, mh := g.mapSize()
	ox, oy := g.cam.Origin(g.player.X, g.player.Y, mw, mh)
	s := g.shifting
	screen.DrawRectShader(w, h, g.transition, &ebiten.DrawRectShaderOptions{
		Images: [4]*ebiten.Image{g.fx[0], g.fx[1]},
		Uniforms: map[string]any{
			"Effect": float32(s.effect),
			"P":      s.ghostness(g.tick),
			"Seed":   s.seed,
			"Center": []float32{float32(screenPx(g.player.X) - ox), float32(screenPx(g.player.Y-charH/3) - oy)},
			"Grid":   []float32{float32(ox % (world.TileSize * zoom)), float32(oy % (world.TileSize * zoom))},
		},
	})
}

// transitionSrc: image 0 is the real picture, image 1 the ghost one; P (0..1) is how far to ghost.
// Effects, by index: shatter, ripple, crt, glitch, wave, iris, dissolve, tiles, card. Sizes are placeholders.
const transitionSrc = `//kage:unit pixels
package main

var Effect float
var P float
var Seed float
var Center vec2 // the player, screen px
var Grid vec2   // the camera's offset within a tile, screen px

func seeReal(p vec2) vec4 { return imageSrc0At(p + imageSrc0Origin()) }
func seeGhost(p vec2) vec4 { return imageSrc1At(p + imageSrc0Origin()) } // in image 0's coordinates
func see(p vec2, ghost bool) vec4 {
	if ghost {
		return seeGhost(p)
	}
	return seeReal(p)
}
func hash(v vec2) float { return fract(sin(dot(v, vec2(12.9898, 78.233))+Seed*91.7) * 43758.5453) }
func black() vec4 { return vec4(0, 0, 0, 1) }
func inside(p, size vec2) bool { return p.x >= 0 && p.y >= 0 && p.x < size.x && p.y < size.y }

// shatter: the real picture cracks into triangle shards that fall away, nearest the player first.
func shatter(at, size vec2) vec4 {
	cell := 80.0
	col := floor(at.x / cell)
	out := seeGhost(at)
	for i := 0; i < 9; i++ {
		row := floor(at.y/cell) - float(i)
		if row >= 0 {
			for k := 0; k < 2; k++ {
				o := vec2(col, row) * cell
				flip := hash(vec2(col, row)+5) >= 0.5 // the cell's diagonal: \ or /
				mid := o + cell*vec2(0.66, 0.33)
				if flip {
					mid = o + cell*vec2(0.33, 0.33)
				}
				if k == 1 {
					mid = o + cell - (mid - o)
				}
				d := distance(mid, Center)/length(size)*0.55 + hash(vec2(col, row)+float(k))*0.1
				t := clamp((P-d)/0.35, 0, 1)
				q := at - vec2(0, t*t*(size.y+cell))
				l := q - o
				upper := l.x >= l.y
				diag := abs(l.x - l.y)
				if flip {
					upper = l.x+l.y < cell
					diag = abs(l.x + l.y - cell)
				}
				if l.x >= 0 && l.y >= 0 && l.x < cell && l.y < cell && upper == (k == 0) {
					c := seeReal(q) * (1 - 0.4*t)
					edge := min(min(l.x, l.y), min(min(cell-l.x, cell-l.y), diag))
					if t == 0 && P > d-0.06 && edge < 1.5 {
						c = vec4(1)
					}
					out = vec4(c.rgb, 1)
				}
			}
		}
	}
	return out
}

// ripple: a ring spreads from the player, warping the picture; the ghost one is inside it.
func ripple(at, size vec2) vec4 {
	d := distance(at, Center)
	x := d - P*(length(size)+160)
	dir := (at - Center) / max(d, 1)
	q := at + dir*14*exp(-x*x/2500)*sin(x*0.12)
	c := see(q, x < 0)
	return vec4(c.rgb+0.25*exp(-x*x/300), 1)
}

// crt: an old TV switching off to a bright line and a dot, then on in the other dimension.
func crt(at, size vec2) vec4 {
	s := abs(1 - 2*P)
	sy := max(smoothstep(0.2, 1, s), 0.004)
	sx := max(smoothstep(0, 0.2, s), 0.003)
	mid := size / 2
	q := mid + (at-mid)/vec2(sx, sy)
	if !inside(q, size) {
		return black()
	}
	c := see(q, P >= 0.5)
	return vec4(mix(c.rgb, vec3(1), pow(1-sy, 4)), 1) // white only as it nears the line
}

// glitch: slices jump sideways and flicker between the two pictures, colors split.
func glitch(at, size vec2) vec4 {
	step := floor(P * 18)
	band := floor(at.y / (8 + 40*hash(vec2(step, 3))))
	k := sin(3.14159 * P)
	q := at + vec2((hash(vec2(band, step+7))-0.5)*90*k, 0)
	ghost := hash(vec2(band, step)) < P
	split := vec2(8*k, 0)
	return vec4(see(q+split, ghost).r, see(q, ghost).g, see(q-split, ghost).b, 1)
}

// wave: rows sway sideways (or columns up and down) while the ghost picture fades in.
func wave(at, size vec2) vec4 {
	k := sin(3.14159 * P)
	off := vec2(28*k*sin(at.y*0.03+P*20), 0)
	if Seed >= 0.5 {
		off = vec2(0, 28*k*sin(at.x*0.03+P*20))
	}
	q := at + off
	return vec4(mix(seeReal(q).rgb, seeGhost(q).rgb, smoothstep(0.3, 0.7, P)), 1)
}

// iris: a circle closes on the player and reopens on the other dimension.
func iris(at, size vec2) vec4 {
	if distance(at, Center) > abs(1-2*P)*length(size) {
		return black()
	}
	return vec4(see(at, P >= 0.5).rgb, 1)
}

// dissolve: 6 px blocks give way to the other picture at random.
func dissolve(at, size vec2) vec4 {
	return vec4(see(at, hash(floor(at/6)) < P).rgb, 1)
}

// tiles: the 16 px tiles turn over one by one in a diagonal wave from the player.
func tiles(at, size vec2) vec4 {
	cell := 48.0
	idx := floor((at + Grid) / cell)
	mid := idx*cell + cell/2 - Grid
	d := (abs(mid.x-Center.x) + abs(mid.y-Center.y)) / (size.x + size.y) * 1.2
	t := clamp((P-d)/0.3, 0, 1)
	s := abs(cos(3.14159 * t))
	lx := at.x - mid.x
	if abs(lx) > cell/2*s {
		return black()
	}
	c := see(vec2(mid.x+lx/max(s, 0.001), at.y), t >= 0.5)
	return vec4(c.rgb*(0.5+0.5*s), 1)
}

// card: the screen turns over on its horizontal axis, the other dimension on its back.
func card(at, size vec2) vec4 {
	s := abs(cos(3.14159 * P))
	mid := size / 2
	q := vec2(at.x, mid.y+(at.y-mid.y)/max(s, 0.001))
	if !inside(q, size) {
		return black()
	}
	return vec4(see(q, P >= 0.5).rgb*(0.5+0.5*s), 1)
}

func Fragment(dst vec4, src vec2, color vec4) vec4 {
	at := src - imageSrc0Origin()
	size := imageSrc0Size()
	e := floor(Effect + 0.5)
	if e == 0 {
		return shatter(at, size)
	}
	if e == 1 {
		return ripple(at, size)
	}
	if e == 2 {
		return crt(at, size)
	}
	if e == 3 {
		return glitch(at, size)
	}
	if e == 4 {
		return wave(at, size)
	}
	if e == 5 {
		return iris(at, size)
	}
	if e == 6 {
		return dissolve(at, size)
	}
	if e == 7 {
		return tiles(at, size)
	}
	return card(at, size)
}
`
