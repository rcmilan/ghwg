package world

import (
	"image"
	"slices"
)

// Path finds a 4-directional route on the cell grid from `from` to `to` (world px).
// It returns tile-center waypoints after the start cell, the last one replaced by `to`; nil when unreachable.
// Cells with any collision rect are avoided. Room doors are solid except the door of Rooms[own]
// (own < 0: every room door is solid). The Entrance is open to everyone.
func (h *Hotel) Path(from, to image.Point, own int) []image.Point {
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
	idx := func(p image.Point) int { return p.Y*m.W + p.X }
	prev := make([]int, m.W*m.H)
	for i := range prev {
		prev[i] = -1
	}
	prev[idx(start)] = idx(start)
	queue := []image.Point{start}
	for len(queue) > 0 && prev[idx(goal)] < 0 {
		c := queue[0]
		queue = queue[1:]
		for _, d := range [...]image.Point{{1, 0}, {-1, 0}, {0, 1}, {0, -1}} {
			n := c.Add(d)
			if !m.inside(n.X, n.Y) || prev[idx(n)] >= 0 || closed[n] || m.Solid(n.X, n.Y) {
				continue
			}
			prev[idx(n)] = idx(c)
			queue = append(queue, n)
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
