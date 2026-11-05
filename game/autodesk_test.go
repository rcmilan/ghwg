package game

import (
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
)

// T069: SPACE at a Room's Door Panel flips it between manual and auto, any time; the Spin keeps it; a new
// Run starts all manual.
func TestDoorPanel(t *testing.T) {
	g, s := headless(t, 3, 3, 1)
	s.hold(ebiten.KeySpace)
	g.Update()
	spot := g.hotel.Rooms[0].PanelSpot()
	g.player.X, g.player.Y = float64(spot.X), float64(spot.Y)
	press := func() {
		s.hold()
		g.Update()
		s.hold(ebiten.KeySpace)
		g.Update()
	}
	press()
	if !g.autoRoom[0] || g.state != StatePlay {
		t.Fatalf("SPACE at Room 0's panel: auto %v, state %d; want auto, still playing", g.autoRoom[0], g.state)
	}
	for i := range g.autoRoom[1:] {
		if g.autoRoom[i+1] {
			t.Fatalf("Room %d flipped too", i+1)
		}
	}
	g.midnight()
	for s.hold(); g.spinning(); g.Update() {
	}
	if !g.autoRoom[0] {
		t.Fatal("the Spin reset Room 0 to manual")
	}
	press()
	if g.autoRoom[0] {
		t.Fatal("a second SPACE left Room 0 on auto")
	}
	press()
	g.startRun()
	if g.autoRoom[0] {
		t.Fatal("a new Run kept Room 0 on auto")
	}
	g.player.X, g.player.Y = float64(g.hotel.Archway.X), float64(g.hotel.Archway.Y)
	press()
	for i, a := range g.autoRoom {
		if a {
			t.Fatalf("SPACE away from every panel flipped Room %d", i)
		}
	}
}

// T072: the Pilot flips a Door Panel by keys alone, and leaves one already set as asked.
func TestPilotSetAuto(t *testing.T) {
	for _, grid := range [][2]int{{2, 1}, {3, 3}} {
		g, _ := headless(t, grid[0], grid[1], 7)
		p := g.autopilot(nop{}, 30, 6).pilot
		run(t, g, 10, func() bool { return g.state == StatePlay })
		room := len(g.hotel.Rooms) - 1
		p.SetAuto(room, true)
		run(t, g, 60*tps, p.Idle)
		if !g.autoRoom[room] || !g.view().Rooms[room].Auto {
			t.Fatalf("%v: after SetAuto(%d, true) the Room is manual", grid, room)
		}
		last := p.lastTap
		p.SetAuto(room, true)
		run(t, g, 60*tps, p.Idle)
		if p.lastTap != last || !g.autoRoom[room] {
			t.Fatalf("%v: SetAuto on a Room already auto pressed a key", grid)
		}
		p.SetAuto(room, false)
		run(t, g, 60*tps, p.Idle)
		if g.autoRoom[room] {
			t.Fatalf("%v: SetAuto(%d, false) left it auto", grid, room)
		}
	}
}
