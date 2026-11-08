package world

import (
	"container/heap"
	"image"
	"slices"
)

// Path costs (D89): a step costs stepCost, plus keepRightCost when the cell to its right (for the
// direction of travel) is open floor too. So paths hug the right-hand side and two-way traffic splits into
// lanes in every Corridor of 2 or more cells, at most ~30% longer than the shortest route.
const (
	stepCost      = 10
	keepRightStep = 3
)

var keepRightCost = keepRightStep // a var so tests can compare against the plain shortest path

// Path finds a 4-directional route on the cell grid from `from` to `to` (world px), keeping right.
// It returns tile-center waypoints after the start cell, the last one replaced by `to`; nil when unreachable.
// Cells with any collision rect are avoided. Room doors are solid except the door of Rooms[own]
// (own < 0: every room door is solid). The Entrance is open to everyone. The cells of the avoid points
// (world px) are solid too: someone standing there.
func (h *Hotel) Path(from, to image.Point, own int, avoid ...image.Point) []image.Point {
	m := h.Map
	start, goal := from.Div(TileSize), to.Div(TileSize)
	if !m.inside(start.X, start.Y) || !m.inside(goal.X, goal.Y) {
		return nil
	}
	closed := map[image.Point]bool{}
	for i, r := range h.Rooms {
		if i != own {
			closed[r.Door], closed[r.Door.Add(image.Pt(0, 1))] = true, true
		}
	}
	for _, p := range avoid {
		closed[p.Div(TileSize)] = true
	}
	open := func(p image.Point) bool { return m.inside(p.X, p.Y) && !closed[p] && !m.Solid(p.X, p.Y) }
	idx := func(p image.Point) int { return p.Y*m.W + p.X }
	pt := func(i int) image.Point { return image.Pt(i%m.W, i/m.W) }
	prev, cost := make([]int, m.W*m.H), make([]int, m.W*m.H)
	for i := range prev {
		prev[i], cost[i] = -1, -1
	}
	prev[idx(start)], cost[idx(start)] = idx(start), 0
	q := &cells{{0, idx(start)}}
	for q.Len() > 0 {
		c := heap.Pop(q).(cell)
		if c.cost > cost[c.at] {
			continue // a stale entry
		}
		if c.at == idx(goal) {
			break
		}
		for _, d := range [...]image.Point{{1, 0}, {-1, 0}, {0, 1}, {0, -1}} {
			n := pt(c.at).Add(d)
			if !open(n) {
				continue
			}
			k := c.cost + stepCost
			if r := n.Add(image.Pt(-d.Y, d.X)); open(r) && m.At(LayerWalls, r.X, r.Y) == 0 { // the right hand (y points down): floor, not a wall's rect-free top row
				k += keepRightCost
			}
			if cost[idx(n)] < 0 || k < cost[idx(n)] {
				prev[idx(n)], cost[idx(n)] = c.at, k
				heap.Push(q, cell{k, idx(n)})
			}
		}
	}
	if prev[idx(goal)] < 0 {
		return nil
	}
	var path []image.Point
	for i := idx(goal); i != idx(start); i = prev[i] {
		path = append(path, image.Pt(i%m.W*TileSize+TileSize/2, i/m.W*TileSize+TileSize/2))
	}
	if len(path) == 0 { // already in the goal cell
		return []image.Point{to}
	}
	slices.Reverse(path)
	path[len(path)-1] = to
	return path
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

func (q cells) Len() int { return len(q) }
func (q cells) Less(i, j int) bool {
	return q[i].cost < q[j].cost || q[i].cost == q[j].cost && q[i].at < q[j].at
}
func (q cells) Swap(i, j int) { q[i], q[j] = q[j], q[i] }
func (q *cells) Push(x any)   { *q = append(*q, x.(cell)) }
func (q *cells) Pop() any {
	old := *q
	x := old[len(old)-1]
	*q = old[:len(old)-1]
	return x
}
