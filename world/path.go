package world

import "image"

// Path costs (D89): a step costs stepCost, plus keepRightCost when the cell to its right (for the
// direction of travel) is open floor too. So paths hug the right-hand side and two-way traffic splits into
// lanes in every Corridor of 2 or more cells, at most ~30% longer than the shortest route.
// The Lobby Archway's side columns are one-way, down on the west and up on the east (F1, 2026-11-14): a step
// the wrong way costs wrongWayCost more, so nobody walks up the column the others walk down, against the
// Hall Wall, unless the rest of the Archway is taken.
const (
	stepCost      = 10
	keepRightStep = 3
	wrongWayCost  = 4 * stepCost
)

var keepRightCost = keepRightStep // a var so tests can compare against the plain shortest path

// Path finds a 4-directional route on the cell grid from `from` to `to` (world px), keeping right.
// It returns tile-center waypoints after the start cell, the last one replaced by `to`; nil when unreachable.
// Cells with any collision rect are avoided. Room doors are solid except the door of Rooms[own]
// (own < 0: every room door is solid). The Entrance is open to everyone. The cells of the avoid points
// (world px) are solid too: someone standing there.
func (h *Hotel) Path(from, to image.Point, own int, avoid ...image.Point) []image.Point {
	m := h.Map
	start, goal := m.Cell(from), m.Cell(to)
	if !m.inside(start.X, start.Y) || !m.inside(goal.X, goal.Y) {
		return nil
	}
	b := h.scratch()
	gen := b.gen
	shut := func(p image.Point) {
		if m.inside(p.X, p.Y) {
			b.shut[p.Y*m.W+p.X] = gen
		}
	}
	for i, r := range h.Rooms {
		if i != own {
			shut(r.Door)
			shut(r.Door.Add(image.Pt(0, 1)))
		}
	}
	for _, p := range avoid {
		shut(m.Cell(p))
	}
	cell := b.cells(m)
	open := func(x, y int) bool {
		return m.inside(x, y) && b.shut[y*m.W+x] != gen && cell[y*m.W+x]&cellOpen != 0
	}
	idx := func(p image.Point) int { return p.Y*m.W + p.X }
	s, g := idx(start), idx(goal)
	// step: what entering cell (nx,ny) by direction d costs.
	step := func(nx, ny int, d image.Point) int {
		k := stepCost
		if rx, ry := nx-d.Y, ny+d.X; open(rx, ry) && cell[ry*m.W+rx]&cellNoWall != 0 { // the right hand (y points down): floor, not a wall's rect-free top row
			k += keepRightCost
		}
		if l := h.l; ny >= l.hall && ny <= l.hall+1 && (d.Y < 0 && nx == l.archway || d.Y > 0 && nx == l.archway+l.archwayW-1) {
			k += wrongWayCost
		}
		return k
	}
	dirs := [...]image.Point{{1, 0}, {-1, 0}, {0, 1}, {0, -1}}
	// A* with the Manhattan distance (every step costs stepCost or more). It runs on past the goal until
	// nothing is left that could lie on an equally cheap route, so every cell such a route can cross holds its
	// exact cost, and the route is then read back below.
	gx, gy := goal.X, goal.Y
	est := func(at int) int {
		dx, dy := at%m.W-gx, at/m.W-gy
		return stepCost * (max(dx, -dx) + max(dy, -dy))
	}
	b.seen[s], b.cost[s] = gen, 0
	q := append(b.q[:0], pack(est(s), s))
	best := -1 // the goal's cost, once known
	for len(q) > 0 {
		c := q[0]
		last := len(q) - 1
		q[0] = q[last]
		q = q[:last]
		q.down(0)
		f, at := int(c>>32), int(uint32(c))
		if best >= 0 && f > best {
			break
		}
		if cost := b.cost[at]; f > cost+est(at) {
			continue // a stale entry
		} else if at == g {
			best = cost
			continue
		}
		cx, cy := at%m.W, at/m.W
		for _, d := range dirs {
			nx, ny := cx+d.X, cy+d.Y
			if !open(nx, ny) {
				continue
			}
			k := b.cost[at] + step(nx, ny, d)
			n := ny*m.W + nx
			if b.seen[n] != gen || k < b.cost[n] {
				b.seen[n], b.cost[n] = gen, k
				q = append(q, pack(k+est(n), n))
				q.up(len(q) - 1)
			}
		}
	}
	b.q = q
	if b.seen[g] != gen {
		return nil
	}
	// Read the route back from the goal: each cell came from its cheapest-entered neighbour, ties to the lower
	// index (the order a plain Dijkstra pops them in, which is what keeps every route the same).
	prev := func(n int) int {
		nx, ny, from, fromCost := n%m.W, n/m.W, -1, 0
		for _, d := range dirs {
			ux, uy := nx-d.X, ny-d.Y
			if !m.inside(ux, uy) {
				continue
			}
			u := uy*m.W + ux
			if b.seen[u] == gen && b.cost[u]+step(nx, ny, d) == b.cost[n] && (from < 0 || b.cost[u] < fromCost || b.cost[u] == fromCost && u < from) {
				from, fromCost = u, b.cost[u]
			}
		}
		return from
	}
	n := 0
	for i := g; i != s; i = prev(i) {
		n++
	}
	if n == 0 { // already in the goal cell
		return []image.Point{to}
	}
	path := make([]image.Point, n)
	for i := g; i != s; i = prev(i) {
		n--
		path[n] = m.Center(i%m.W, i/m.W)
	}
	path[len(path)-1] = to
	return path
}

// pathBuf: Path's working memory, reused call to call. A cell's entry counts only when it holds the call's
// generation, so nothing is cleared between calls.
type pathBuf struct {
	gen               uint32
	seen, shut, known []uint32
	cost              []int
	solid             []bool
	q                 cells
	flags             []uint8 // per cell: cellOpen, cellNoWall, as of flagsRev
	flagsRev          int
	doors             []int  // UpdateDoors' scratch: door indexes by top edge
	want              []bool // ... and which doors an actor asks open
}

// Static facts of a cell, remembered until the next Set (like Tilemap.solidFast).
const (
	cellOpen   = 1 // no collision rect
	cellNoWall = 2 // no wall tile: floor, not a wall's rect-free top row
)

func (b *pathBuf) cells(m *Tilemap) []uint8 {
	if len(b.flags) != m.W*m.H || b.flagsRev != m.Rev+1 {
		b.flags, b.flagsRev = make([]uint8, m.W*m.H), m.Rev+1
		for i := range b.flags {
			x, y := i%m.W, i/m.W
			if !m.Solid(x, y) {
				b.flags[i] |= cellOpen
			}
			if m.At(LayerWalls, x, y) == 0 {
				b.flags[i] |= cellNoWall
			}
		}
	}
	return b.flags
}

func (h *Hotel) scratch() *pathBuf {
	b, n := &h.pf, h.Map.W*h.Map.H
	if len(b.seen) != n {
		*b = pathBuf{seen: make([]uint32, n), shut: make([]uint32, n), cost: make([]int, n)}
	}
	b.gen++
	return b
}

// Choke reports whether cell c is a 1-cell choke point (D89): open, with solid cells on both sides
// across one axis, like a Room doorway.
func (h *Hotel) Choke(c image.Point) bool {
	m := h.Map
	f := h.pf.cells(m)
	solid := func(x, y int) bool { return !m.inside(x, y) || f[y*m.W+x]&cellOpen == 0 }
	return !solid(c.X, c.Y) && (solid(c.X-1, c.Y) && solid(c.X+1, c.Y) || solid(c.X, c.Y-1) && solid(c.X, c.Y+1))
}

// cells is a min-heap of cells packed as cost<<32 | index, so one compare orders by path cost, ties by cell
// index (paths are deterministic).
type cells []uint64

func pack(cost, at int) uint64 { return uint64(cost)<<32 | uint64(at) }

func (q cells) less(i, j int) bool { return q[i] < q[j] }

func (q cells) up(j int) {
	for j > 0 {
		i := (j - 1) / 2
		if !q.less(j, i) {
			break
		}
		q[i], q[j] = q[j], q[i]
		j = i
	}
}

func (q cells) down(i int) {
	for {
		j := 2*i + 1
		if j >= len(q) {
			return
		}
		if j+1 < len(q) && q.less(j+1, j) {
			j++
		}
		if !q.less(j, i) {
			return
		}
		q[i], q[j] = q[j], q[i]
		i = j
	}
}
