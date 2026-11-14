package world

import (
	"image"
	"testing"
)

// D89, T022: in the 2-row Corridor above the Hall Wall, an eastbound and a westbound path keep right,
// each to its own row, and no path runs much longer than the shortest one.
func TestKeepRight(t *testing.T) {
	forGrids(t, func(t *testing.T, h *Hotel, l layout) {
		west, east := cellCenter(2, l.hall-2), cellCenter(l.w-3, l.hall-1)
		for _, c := range []struct {
			name     string
			from, to image.Point
			row      int // the right-hand row
		}{{"eastbound", west, east, l.hall - 1}, {"westbound", east, west, l.hall - 2}} {
			for _, p := range h.Path(c.from, c.to, -1) {
				if x, y := p.X/TileSize, p.Y/TileSize; x > 4 && x < l.w-6 && y != c.row {
					t.Errorf("%s: cell (%d,%d) is off its right-hand row %d", c.name, x, y, c.row)
				}
			}
		}
		for i, r := range h.Rooms {
			for _, s := range r.Stays {
				keep := len(h.Path(h.Spawn, s, i))
				keepRightCost = 0
				shortest := len(h.Path(h.Spawn, s, i))
				keepRightCost = keepRightStep
				if keep > shortest+4 {
					t.Errorf("room %s: Spawn to its Stay Spot takes %d cells keeping right, %d at the shortest", r.Number, keep, shortest)
				}
			}
		}
	})
}

// D89, T023: Room doorways are 1-cell choke points; the open Corridors and the Lobby aren't.
// F1: through the Lobby Archway, a walk up to a west Room and a walk down from it use different columns.
func TestArchwayOneWay(t *testing.T) {
	forGrids(t, func(t *testing.T, h *Hotel, l layout) {
		col := func(path []image.Point) int {
			for _, p := range path {
				if c := p.Div(TileSize); c.Y == l.hall+1 {
					return c.X
				}
			}
			return -1
		}
		for i, r := range h.Rooms {
			up, down := col(h.Path(h.Spawn, r.Stays[0], i)), col(h.Path(r.Stays[0], h.Exit, i))
			if up < 0 || down < 0 || up == l.archway || down == l.archway+l.archwayW-1 || up == down {
				t.Errorf("Room %s: up the Archway in column %d, down in %d (Archway %d..%d)", r.Number, up, down, l.archway, l.archway+l.archwayW-1)
			}
		}
	})
}

func TestChoke(t *testing.T) {
	forGrids(t, func(t *testing.T, h *Hotel, l layout) {
		for _, r := range h.Rooms {
			if !h.Choke(r.Door.Add(image.Pt(0, 1))) { // the door's bottom half: the wall's top row has no rect
				t.Errorf("room %s's doorway isn't a choke point", r.Number)
			}
		}
		for _, c := range []image.Point{{2, l.hall - 1}, {l.w / 2, l.hall - 2}, {l.w / 2, l.counter + 2}} {
			if h.Choke(c) {
				t.Errorf("cell %v in an open area is a choke point", c)
			}
		}
	})
}
