package game

import (
	"math"
	"slices"
	"testing"

	"ghosthotel/entities"
	"ghosthotel/world"
)

// T208 (D185): the 2×1 map's wall runs, each the stretch of a face's bottom row where the whole face shows,
// to the pixel (2026-11-08): the wall's hitbox, trimmed by the side lines' (6 px) and cut by the Archway,
// carried across doors; and their links, through the side lines their ends touch: none at the Archway, whose
// ends are open, so nothing blinks across it. Every Room's back wall reaches every other, on every grid.
func TestWallGraph(t *testing.T) {
	g, _ := headless(t, 2, 1, 1)
	g.startRun()
	type span struct{ row, x0, x1 int }
	want := []span{
		{2, 22, 154}, {2, 160, 192}, {2, 198, 330}, // the north wall: Room 101's back wall, the Corridor's, Room 102's
		{9, 22, 154}, {9, 198, 330}, // the Rooms' south faces, across their doors
		{13, 22, 144}, {13, 208, 330}, // the Hall Wall: the Archway makes it two walls
		{21, 22, 330}, // the front face, across the Entrance
	}
	links := [][]int{{1, 3}, {0, 2, 3}, {1, 4}, {0, 1, 5}, {2, 6}, {3, 7}, {4, 7}, {5, 6}} // down the outer walls, across behind the Corridor's side lines
	if len(g.walls) != len(want) {
		t.Fatalf("%d runs, want %d: %+v", len(g.walls), len(want), g.walls)
	}
	for i, w := range want {
		r := g.walls[i]
		if r.x0 != float64(w.x0) || r.x1 != float64(w.x1) || r.y != float64((w.row+1)*world.TileSize)-entities.FootH/2 {
			t.Errorf("run %d: x %v..%v y %v, want px %d..%d of row %d", i, r.x0, r.x1, r.y, w.x0, w.x1, w.row)
		}
		if got := slices.Sorted(slices.Values(r.links)); !slices.Equal(got, links[i]) {
			t.Errorf("run %d links %v, want %v", i, got, links[i])
		}
	}
	for _, grid := range [][2]int{{2, 1}, {2, 2}, {3, 3}} {
		g, _ := headless(t, grid[0], grid[1], 1)
		g.startRun()
		for a := range g.hotel.Rooms {
			for b := range g.hotel.Rooms {
				from, _, _ := g.backWall(a)
				to, _, _ := g.backWall(b)
				if p := g.wallRoute(from, to); len(p) == 0 && to != from {
					t.Fatalf("%dx%d: no route from Room %d's back wall to Room %d's", grid[0], grid[1], a, b)
				}
			}
		}
	}
}

// T209 (D185): a Shadow sent from the front face to a Room's back wall runs at shadowSpeed along each
// run, only ever on a run, and teleports vertically: x holds while it fades out, blinks onto the next run
// and fades in. It arrives at its goal.
func TestShadowTravel(t *testing.T) {
	g, _ := headless(t, 2, 1, 1)
	g.startRun()
	if p := g.wallRoute(7, 0); len(p) != 3 || p[0] != 5 || p[2] != 0 {
		t.Fatalf("route from the front face to Room 101's back wall: %v, want via the Hall Wall's west run, its south face, and 0", p)
	}
	lo, _ := g.walls[7].ends()
	sh := &shade{run: 7, x: lo, essence: shadowEssence, anim: entities.NewAnim()}
	g.sendShade(sh, 0, 300) // past the wall's end: it stops touching it
	teleports := 0
	for i, arrived := 0, false; !arrived; i++ {
		if i > 2000 {
			t.Fatal("it never arrived")
		}
		run, x, fade := sh.run, sh.x, sh.fade
		arrived = g.move(sh)
		if lo, hi := g.walls[sh.run].ends(); sh.x < lo || sh.x > hi {
			t.Fatalf("frame %d: x %v past run %d's ends (%v..%v): part of it is off the wall", i, sh.x, sh.run, lo, hi)
		}
		switch {
		case sh.run != run:
			teleports++
			if lo, hi := g.blinkSpots(g.walls[run], g.walls[sh.run]); x != lo && x != hi {
				t.Fatalf("blinked at x %v, want a wall end of the shared stretch: %v or %v", x, lo, hi)
			}
			if sh.x != x || fade != teleportFrames/2-1 || g.shadowAlpha(sh) > 0.1 {
				t.Fatalf("a teleport at fade %d: x %v → %v, alpha %.2f; want a vertical blink, faded out, at half", fade, x, sh.x, g.shadowAlpha(sh))
			}
		case fade > 0:
			if sh.x != x {
				t.Fatalf("fading, x moved %v → %v", x, sh.x)
			}
		case math.Abs(sh.x-x) > shadowSpeed:
			t.Fatalf("ran %v px in a frame, want at most shadowSpeed", sh.x-x)
		}
	}
	if _, end := g.walls[0].ends(); teleports != 3 || sh.run != 0 || sh.x != end || sh.fade != 0 {
		t.Fatalf("arrived on run %d at x %v after %d teleports; want run 0 at its east end %v (sent past it, to 300), 3", sh.run, sh.x, teleports, end)
	}
}

// A roaming Shadow walks whole walls: each time it stops, it's at one of its wall's ends, never midway, and
// only crosses a hidden wall (the Corridor's back wall, too narrow to show one) (2026-11-08).
func TestWander(t *testing.T) {
	g, _ := headless(t, 2, 1, 1)
	g.startRun()
	for k, r := range g.walls {
		if r.hidden() != (k == 1) {
			t.Fatalf("run %d (%v px) hidden %v; want only the Corridor's back wall", k, r.x1-r.x0, r.hidden())
		}
	}
	g.DevHaunt(1)
	sh := g.shades[0][0]
	runs := map[int]bool{}
	for i := 0; i < 20000; i++ {
		if g.move(sh) {
			if lo, hi := g.walls[sh.run].ends(); i > 0 && sh.x != lo && sh.x != hi {
				t.Fatalf("frame %d: stopped at x %v on run %d (%v..%v); want at an end", i, sh.x, sh.run, lo, hi)
			}
			runs[sh.run] = true
			if g.wander(sh); g.walls[sh.run].hidden() && len(sh.path) == 0 {
				t.Fatalf("frame %d: turned back on the hidden run %d; want it only crossed", i, sh.run)
			}
		}
	}
	if len(runs) < len(g.walls)-1 {
		t.Fatalf("roamed %d walls of %d", len(runs), len(g.walls)-1)
	}
}
