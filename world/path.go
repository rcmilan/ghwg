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
	open := func(x, y int) bool {
		return m.inside(x, y) && b.shut[y*m.W+x] != gen && !m.solidFast(x, y)
	}
	idx := func(p image.Point) int { return p.Y*m.W + p.X }
	s, g := idx(start), idx(goal)
	b.seen[s], b.cost[s], b.prev[s] = gen, 0, s
	q := append(b.q[:0], cell{0, s})
	for len(q) > 0 {
		c := q[0]
		last := len(q) - 1
		q[0] = q[last]
		q = q[:last]
		q.down(0)
		if c.cost > b.cost[c.at] {
			continue // a stale entry
		}
		if c.at == g {
			break
		}
		cx, cy := c.at%m.W, c.at/m.W
		for _, d := range [...]image.Point{{1, 0}, {-1, 0}, {0, 1}, {0, -1}} {
			nx, ny := cx+d.X, cy+d.Y
			if !open(nx, ny) {
				continue
			}
			k := c.cost + stepCost
			if rx, ry := nx-d.Y, ny+d.X; open(rx, ry) && m.At(LayerWalls, rx, ry) == 0 { // the right hand (y points down): floor, not a wall's rect-free top row
				k += keepRightCost
			}
			if l := h.l; ny >= l.hall && ny <= l.hall+1 && (d.Y < 0 && nx == l.archway || d.Y > 0 && nx == l.archway+l.archwayW-1) {
				k += wrongWayCost
			}
			n := ny*m.W + nx
			if b.seen[n] != gen || k < b.cost[n] {
				b.seen[n], b.cost[n], b.prev[n] = gen, k, c.at
				q = append(q, cell{k, n})
				q.up(len(q) - 1)
			}
		}
	}
	b.q = q
	if b.seen[g] != gen {
		return nil
	}
	n := 0
	for i := g; i != s; i = int(b.prev[i]) {
		n++
	}
	if n == 0 { // already in the goal cell
		return []image.Point{to}
	}
	path := make([]image.Point, n)
	for i := g; i != s; i = int(b.prev[i]) {
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
	cost, prev        []int
	solid             []bool
	q                 cells
}

func (h *Hotel) scratch() *pathBuf {
	b, n := &h.pf, h.Map.W*h.Map.H
	if len(b.seen) != n {
		*b = pathBuf{seen: make([]uint32, n), shut: make([]uint32, n), cost: make([]int, n), prev: make([]int, n)}
	}
	b.gen++
	return b
}

// Choke reports whether cell c is a 1-cell choke point (D89): open, with solid cells on both sides
// across one axis, like a Room doorway.
func (h *Hotel) Choke(c image.Point) bool {
	m := h.Map
	return !m.Solid(c.X, c.Y) && (m.Solid(c.X-1, c.Y) && m.Solid(c.X+1, c.Y) || m.Solid(c.X, c.Y-1) && m.Solid(c.X, c.Y+1))
}

// cells is a min-heap of cells by path cost, ties by cell index so paths are deterministic.
type cell struct{ cost, at int }
type cells []cell

func (q cells) less(i, j int) bool {
	return q[i].cost < q[j].cost || q[i].cost == q[j].cost && q[i].at < q[j].at
}

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
