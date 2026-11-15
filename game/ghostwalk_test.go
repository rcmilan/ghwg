package game

import (
	"image"
	"testing"

	"ghosthotel/entities"
	"ghosthotel/world"
)

// grown33 is a 3x3 hotel's Run, wall-walking on sale.
func grown33(t *testing.T) *Game {
	t.Helper()
	g, _ := headless(t, 3, 3, 1)
	g.startRun()
	return g
}

// walk moves the Concierge n frames along (dx,0).
func walk(g *Game, dx float64, n int) {
	for range n {
		g.step(dx, 0)
	}
}

// B1-W: Wall-walking is on sale only once the hotel is 3x3 (grid0 smaller), after the Mirror.
func TestWallWalkOnSale(t *testing.T) {
	g := grown33(t)
	i := row(t, WallWalk)
	g.unlock(Mirror)
	g.grid0 = [2]int{2, 1}
	if !g.onSale(i) {
		t.Error("not on sale at 3x3")
	}
	g2, _ := headless(t, 2, 2, 1)
	g2.startRun()
	g2.unlock(Mirror)
	if g2.onSale(row(t, WallWalk)) {
		t.Error("on sale in a 2x2 hotel")
	}
	if !g.locked[WallWalk] {
		t.Error("a new Run starts with wall-walking locked")
	}
}

// B1-W: in ghost with wall-walking he passes a Room's side wall; in real, or without it, he doesn't; the outer
// wall stays solid.
func TestWallWalk(t *testing.T) {
	g := grown33(t)
	ts := world.TileSize
	var mid world.Room
	for _, r := range g.hotel.Rooms {
		if r.Origin.X > 1 && r.Origin.Y > 1 {
			mid = r
			break
		}
	}
	y := float64((mid.Origin.Y+RoomMidRow)*ts + ts/2)
	startX := float64((mid.Origin.X-1)*ts + ts/2) // the corridor west of it
	try := func(ghost, owned bool) float64 {
		g.ghost = ghost
		if owned {
			g.unlock(WallWalk)
		} else {
			g.locked[WallWalk] = true
		}
		g.player.X, g.player.Y = startX, y
		walk(g, 1, 120)
		return g.player.X
	}
	in := float64(mid.Origin.X*ts + 3*ts)
	if x := try(true, true); x < in {
		t.Errorf("ghost with wall-walking stopped at x=%.0f, want past %.0f", x, in)
	}
	for _, c := range [][2]bool{{false, true}, {true, false}} {
		if x := try(c[0], c[1]); x >= float64(mid.Origin.X*ts) {
			t.Errorf("ghost=%v owned=%v passed the wall: x=%.0f", c[0], c[1], x)
		}
	}
	// the outer west wall of a left-column Room
	var left world.Room
	for _, r := range g.hotel.Rooms {
		if r.Origin.X == 1 {
			left = r
			break
		}
	}
	west := func(ghost bool) float64 {
		g.ghost = ghost
		g.unlock(WallWalk)
		g.player.X, g.player.Y = float64(left.Origin.X*ts+3*ts), float64((left.Origin.Y+RoomMidRow)*ts+ts/2)
		walk(g, -1, 120)
		return g.player.X
	}
	if gx, rx := west(true), west(false); gx != rx {
		t.Errorf("the outer wall stops him at x=%.0f in ghost, %.0f in real", gx, rx)
	}
}

// RoomMidRow: a row of a Room with only walls at its sides (no furniture rows assumed): the middle.
const RoomMidRow = world.RoomH / 2

// B1-W: shifting back to real from inside a wall nudges him to open floor.
func TestWallWalkUnstick(t *testing.T) {
	g := grown33(t)
	ts := world.TileSize
	var mid world.Room
	for _, r := range g.hotel.Rooms {
		if r.Origin.X > 1 && r.Origin.Y > 1 {
			mid = r
			break
		}
	}
	g.ghost = true
	g.unlock(WallWalk)
	for dy := range world.RoomH { // some row of the west wall whose cell is solid
		x, y := mid.Origin.X*ts+ts/2, (mid.Origin.Y+dy)*ts+ts/2
		g.player.X, g.player.Y = float64(x), float64(y)
		if !g.blocked(entities.FootBox(g.player.X, g.player.Y)) {
			continue
		}
		g.ghost = true
		g.startShift()
		for range flipFrames + 2 {
			g.step(0, 0)
			g.updateShift()
		}
		if g.ghost || g.blocked(entities.FootBox(g.player.X, g.player.Y)) && !g.inLane(entities.FootBox(g.player.X, g.player.Y)) {
			t.Fatalf("row %d: after shifting back ghost=%v, still stuck at %v", dy, g.ghost, image.Pt(int(g.player.X), int(g.player.Y)))
		}
		return
	}
	t.Fatal("no solid wall cell found")
}
