package world

import (
	"image"
	"slices"
	"testing"
)

// refPath: Path as a plain Dijkstra (the search A* replaced), popping the cheapest cell, ties to the lower index.
func refPath(h *Hotel, from, to image.Point, own int, avoid ...image.Point) []image.Point {
	m := h.Map
	start, goal := m.Cell(from), m.Cell(to)
	shut := map[image.Point]bool{}
	for i, r := range h.Rooms {
		if i != own {
			shut[r.Door], shut[r.Door.Add(image.Pt(0, 1))] = true, true
		}
	}
	for _, p := range avoid {
		shut[m.Cell(p)] = true
	}
	open := func(x, y int) bool { return m.inside(x, y) && !shut[image.Pt(x, y)] && !m.Solid(x, y) }
	n := m.W * m.H
	cost, prev, done := make([]int, n), make([]int, n), make([]bool, n)
	for i := range cost {
		cost[i] = -1
	}
	s, g := start.Y*m.W+start.X, goal.Y*m.W+goal.X
	cost[s], prev[s] = 0, s
	for {
		at := -1
		for i := range cost {
			if !done[i] && cost[i] >= 0 && (at < 0 || cost[i] < cost[at]) {
				at = i
			}
		}
		if at < 0 || at == g {
			break
		}
		done[at] = true
		for _, d := range [...]image.Point{{1, 0}, {-1, 0}, {0, 1}, {0, -1}} {
			nx, ny := at%m.W+d.X, at/m.W+d.Y
			if !open(nx, ny) {
				continue
			}
			k := cost[at] + stepCost
			if rx, ry := nx-d.Y, ny+d.X; open(rx, ry) && m.At(LayerWalls, rx, ry) == 0 {
				k += keepRightCost
			}
			if l := h.l; ny >= l.hall && ny <= l.hall+1 && (d.Y < 0 && nx == l.archway || d.Y > 0 && nx == l.archway+l.archwayW-1) {
				k += wrongWayCost
			}
			if j := ny*m.W + nx; cost[j] < 0 || k < cost[j] {
				cost[j], prev[j] = k, at
			}
		}
	}
	if cost[g] < 0 {
		return nil
	}
	var path []image.Point
	for i := g; i != s; i = prev[i] {
		path = append(path, m.Center(i%m.W, i/m.W))
	}
	if len(path) == 0 {
		return []image.Point{to}
	}
	slices.Reverse(path)
	path[len(path)-1] = to
	return path
}

// Path is an A* that reads its route back to match Dijkstra's: the same waypoints for any start, goal and
// avoided cells, including ones that cannot be reached.
func TestPathMatchesDijkstra(t *testing.T) {
	forGrids(t, func(t *testing.T, h *Hotel, l layout) {
		var spots []image.Point
		for _, r := range h.Rooms {
			spots = append(spots, r.Stays...)
		}
		spots = append(spots, h.Spawn, h.Archway, cellCenter(2, l.hall-2), cellCenter(l.w-3, l.hall-1))
		for i, from := range spots {
			for j := (i * 7) % 5; j < len(spots); j += 5 {
				to := spots[j]
				for _, avoid := range [][]image.Point{nil, {spots[(i+j)%len(spots)]}, {cellCenter(l.archway, l.hall), cellCenter(l.archway+1, l.hall)}} {
					for _, own := range []int{-1, i % len(h.Rooms)} {
						if got, want := h.Path(from, to, own, avoid...), refPath(h, from, to, own, avoid...); !slices.Equal(got, want) {
							t.Fatalf("Path %v -> %v own %d avoid %v:\n got %v\nwant %v", from, to, own, avoid, got, want)
						}
					}
				}
			}
		}
	})
}
