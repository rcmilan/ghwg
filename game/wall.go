package game

import (
	"math"
	"slices"
	"strings"

	"ghosthotel/entities"
	"ghosthotel/world"
)

// The wall graph (D185): Shadow People stand only on horizontal wall runs and blink between them vertically.
const teleportFrames = 24 // (placeholder) a vertical teleport: half fading out, half fading in

// wallRun is a continuous stretch of wall face, to the pixel: the face's hitbox minus the side lines'
// (2026-11-08: a Shadow stops where the wall does, as at the Archway). A Shadow on it has its feet at y and
// its whole body inside x0..x1 (world px), x in ends(). end: the side line each end touches (its left px),
// -1 at an opening such as the Archway. links: the runs it blinks to.
type wallRun struct {
	x0, x1, y float64
	end       [2]int
	links     []int
}

// hidden: too narrow to show a Shadow; one only passes through it, unseen.
func (r wallRun) hidden() bool { return r.x1-r.x0 < shadowMinWall }

// ends: where a Shadow's x may go on r; at either one it touches the wall's end, not a pixel past it.
func (r wallRun) ends() (float64, float64) { return r.x0 + charW/2, r.x1 - charW/2 }

// wallRuns lists every run of m, top to bottom then left to right, and cells: each cell's run (its face's
// two rows), -1 if none. A run is a face's bottom row where both rows show wall, carried across a door (the
// Shadow passes behind it) but not an open gap such as the Archway; trimmed off by the side lines' hitboxes;
// at least a body wide. Shadows go across on the walls and up and down inside the side walls (2026-11-08):
// each side line links the runs whose ends touch it, each to the next along it. A blink is at a wall's end,
// never mid-wall, never across the Archway.
func wallRuns(m *world.Tilemap) (runs []wallRun, cells []int) {
	const ts = world.TileSize
	tile := func(l, x, y int) world.Tile { return m.TS.Tiles[m.At(l, x, y)] }
	door := func(x, y int) bool { return tile(world.LayerDecor, x, y).Door || tile(world.LayerWalls, x, y).Door }
	face := func(x, y int) bool {
		n := tile(world.LayerWalls, x, y).Name
		return strings.HasPrefix(n, "wall_") && n != "wall_lintel"
	}
	foot := func(x, y int) bool {
		return y > 0 && (face(x, y) && face(x, y-1) && strings.HasSuffix(tile(world.LayerWalls, x, y).Name, "_bottom") || door(x, y) && door(x, y-1))
	}
	// span: the world-px part of foot cell (x,y) that no side line's hitbox covers, on either face row
	span := func(x, y int) (a, b int) {
		a, b = x*ts, (x+1)*ts
		for _, c := range [2]int{y - 1, y} {
			for _, l := range [2]int{world.LayerWalls, world.LayerDecor} {
				if t := tile(l, x, c); strings.HasPrefix(t.Name, "border_") {
					for _, s := range t.Solid {
						if s.Min.X == 0 {
							a = max(a, x*ts+s.Max.X)
						} else {
							b = min(b, x*ts+s.Min.X)
						}
					}
				}
			}
		}
		return a, b
	}
	for y := range m.H {
		fy := float64((y+1)*ts) - entities.FootH/2
		for x := range m.W {
			if a, b := span(x, y); foot(x, y) && a < b {
				if n := len(runs) - 1; n >= 0 && runs[n].y == fy && runs[n].x1 == float64(a) {
					runs[n].x1 = float64(b)
				} else {
					runs = append(runs, wallRun{x0: float64(a), x1: float64(b), y: fy})
				}
			}
		}
	}
	runs = slices.DeleteFunc(runs, func(r wallRun) bool { return r.x1-r.x0 < charW })
	// line: the left px of the side line whose hitbox covers px column px on face row y or the row above
	line := func(px, y int) int {
		x := px / ts
		for _, c := range [2]int{y - 1, y} {
			for _, l := range [2]int{world.LayerWalls, world.LayerDecor} {
				if t := tile(l, x, c); strings.HasPrefix(t.Name, "border_") {
					for _, s := range t.Solid {
						if x*ts+s.Min.X <= px && px < x*ts+s.Max.X {
							return x*ts + s.Min.X
						}
					}
				}
			}
		}
		return -1
	}
	type touch struct{ line, run int }
	var touches []touch // in run order: down each line, then across it
	for k := range runs {
		r := &runs[k]
		row := int(r.y) / ts
		r.end = [2]int{line(int(r.x0)-1, row), line(int(r.x1), row)}
		for _, e := range r.end {
			if e >= 0 {
				touches = append(touches, touch{e, k})
			}
		}
	}
	slices.SortStableFunc(touches, func(a, b touch) int { return a.line - b.line })
	for i := 1; i < len(touches); i++ {
		if a, b := touches[i-1], touches[i]; a.line == b.line && a.run != b.run && !slices.Contains(runs[a.run].links, b.run) {
			runs[a.run].links = append(runs[a.run].links, b.run)
			runs[b.run].links = append(runs[b.run].links, a.run)
		}
	}
	cells = make([]int, m.W*m.H)
	for i := range cells {
		cells[i] = -1
	}
	for k, r := range runs { // ponytail: one run per cell; side lines sit on cell edges, so none share one
		row := int(r.y) / ts
		for x := int(r.x0) / ts; x*ts < int(r.x1); x++ {
			cells[row*m.W+x], cells[(row-1)*m.W+x] = k, k
		}
	}
	return runs, cells
}

// runAt: the run nearest world point (x,y).
func (g *Game) runAt(x, y float64) int {
	best, far := 0, math.Inf(1)
	for i, r := range g.walls {
		if d := math.Hypot(x-clamp(x, r.x0, r.x1), y-r.y); d < far {
			best, far = i, d
		}
	}
	return best
}

// wallRoute: the runs to teleport through from run from to run to, to included; fewest teleports.
func (g *Game) wallRoute(from, to int) []int {
	prev := make([]int, len(g.walls))
	for i := range prev {
		prev[i] = -1
	}
	prev[from] = from
	for q := []int{from}; len(q) > 0; q = q[1:] {
		for _, n := range g.walls[q[0]].links {
			if prev[n] < 0 {
				prev[n], q = q[0], append(q, n)
			}
		}
	}
	if prev[to] < 0 {
		return nil // unreachable: a run no wall shares an end with, such as the Corridor's back wall
	}
	var path []int
	for r := to; r != from; r = prev[r] {
		path = append([]int{r}, path...)
	}
	return path
}

// sendShade sets a Shadow off to x on run to.
func (g *Game) sendShade(sh *shade, to int, x float64) {
	if sh.path = g.wallRoute(sh.run, to); sh.path == nil {
		to = sh.run // already there, or out of reach: it stays on its own wall
	}
	lo, hi := g.walls[to].ends()
	sh.goal = clamp(x, lo, hi)
}

// move takes a Shadow one frame along its way: it runs at shadowSpeed to the nearer blink spot toward the
// next run, fades out, blinks onto it at the same x, and fades in. True once it stands at its goal.
func (g *Game) move(sh *shade) bool {
	r := g.walls[sh.run]
	defer func() {
		sh.anim.Update(sh.x, g.walls[sh.run].y)
		sh.anim.Facing = entities.Toward(sh.dir, 0)
	}()
	if sh.fade > 0 {
		if sh.fade++; sh.fade == teleportFrames/2 {
			sh.run, sh.path = sh.path[0], sh.path[1:]
			lo, hi := g.walls[sh.run].ends()
			sh.x = clamp(sh.x, lo, hi)
		}
		if sh.fade >= teleportFrames {
			sh.fade = 0
		}
		return false
	}
	to := sh.goal
	if len(sh.path) > 0 {
		lo, hi := g.blinkSpots(r, g.walls[sh.path[0]])
		if to = lo; hi-sh.x < sh.x-lo {
			to = hi
		}
	}
	speed := shadowSpeed
	if sh.patrol {
		speed = patrolSpeed
	}
	if d := to - sh.x; math.Abs(d) > speed {
		sh.x, sh.dir = sh.x+math.Copysign(speed, d), math.Copysign(1, d)
		return false
	} else if d != 0 {
		sh.dir = math.Copysign(1, d)
	}
	sh.x = to
	if len(sh.path) > 0 {
		sh.fade = 1
		return false
	}
	return true
}

// wander sends a roaming Shadow on from where it stands, always a whole wall, end to end (2026-11-08): at an
// end it blinks onto a wall sharing that end's side line, or turns back; a hidden wall it only crosses.
func (g *Game) wander(sh *shade) {
	r := g.walls[sh.run]
	var next []int
	for _, n := range r.links {
		if a, b := g.blinkSpots(r, g.walls[n]); a == sh.x || b == sh.x {
			next = append(next, n)
		}
	}
	if !r.hidden() || len(next) == 0 {
		next = append(next, sh.run)
	}
	n := next[g.chat.IntN(len(next))]
	lo, hi := g.walls[n].ends()
	if math.Abs(lo-sh.x) > math.Abs(hi-sh.x) {
		hi = lo
	}
	g.sendShade(sh, n, hi) // its far end
}

// blinkSpots: where a Shadow blinks from run r to linked run n: r's ends on a side line n touches too, a
// half body in (2026-11-08: only at wall ends, like stairs). One such end: both are it.
func (g *Game) blinkSpots(r, n wallRun) (lo, hi float64) {
	lo, hi = r.ends()
	on := func(l int) bool { return l >= 0 && (l == n.end[0] || l == n.end[1]) }
	if !on(r.end[0]) {
		lo = hi
	}
	if !on(r.end[1]) {
		hi = lo
	}
	return lo, hi
}

func clamp(x, lo, hi float64) float64 { return max(lo, min(hi, x)) }
