package game

import (
	"fmt"
	"image"
	"math"
	"slices"
	"testing"

	"ghosthotel/world"
)

// D71–D74: a Run starts on Day 1 at 08:00, an hour is 600 frames, and T runs the world (the Clock too) ×fastScale.
func TestClock(t *testing.T) {
	if got := clockText(2*dayFrames + 14*hourFrames + 20*minuteFrames); got != "DAY 3  14:20" {
		t.Errorf("clockText = %q, want DAY 3  14:20", got)
	}
	g := mustNew(t)
	g.startRun()
	if got := clockText(g.clock); got != "DAY 1  08:00" {
		t.Fatalf("Run starts at %q, want DAY 1  08:00", got)
	}
	for range hourFrames {
		g.step(0, 0)
	}
	if got := clockText(g.clock); got != "DAY 1  09:00" {
		t.Errorf("600 frames later the clock reads %q, want DAY 1  09:00", got)
	}
	c := g.clock
	g.fast = true
	g.step(0, 0)
	if g.clock != c+fastScale {
		t.Errorf("holding T ran the clock %d frames in one step, want %d", g.clock-c, fastScale)
	}
}

// D1 (T316): at a quarter of the time scale a game minute takes four times the frames, and the Concierge and a
// walking Guest cover a quarter of the ground a frame.
func TestSlowScale(t *testing.T) {
	g, _ := headless(t, 2, 1, 1)
	g.startRun()
	g.scale = 0.25
	c := g.clock
	for range 4*minuteFrames - 1 {
		g.step(0, 0)
	}
	if g.clock != c+minuteFrames-1 {
		t.Fatalf("%d frames at scale 0.25 ran the clock %d, want %d", 4*minuteFrames-1, g.clock-c, minuteFrames-1)
	}
	if g.step(0, 0); g.clock != c+minuteFrames {
		t.Fatalf("a game minute at scale 0.25 took more than %d frames", 4*minuteFrames)
	}
	g.player.X, g.player.Y = float64(g.hotel.Archway.X), float64(g.hotel.Archway.Y) // open floor
	x := g.player.X
	if g.step(1, 0); g.player.X-x != conciergeSpeed/4 {
		t.Errorf("the Concierge walked %v px in a frame at scale 0.25, want %v", g.player.X-x, conciergeSpeed/4)
	}
	r := g.hotel.Rooms[0]
	y := r.Door.Y + 3
	cell := func(dx int) image.Point {
		return image.Pt((r.Door.X+dx)*world.TileSize+world.TileSize/2, y*world.TileSize+world.TileSize/2)
	}
	guest := lonePartyAt(g, Human, 0, cell(-2), cell(6))
	gx, gy := guest.X, guest.Y
	if g.step(0, 0); math.Abs(math.Hypot(guest.X-gx, guest.Y-gy)-arriveSpeed/4) > 1e-9 {
		t.Errorf("a Guest walked %v px in a frame at scale 0.25, want %v", math.Hypot(guest.X-gx, guest.Y-gy), arriveSpeed/4)
	}
}

// D1 (T316): slowed down, the busy hotel still flows: nobody walking gets nowhere for stuckFrames of world time.
func TestSlowBusy(t *testing.T) {
	g, _ := headless(t, 2, 1, 1)
	g.startRun()
	g.DevBusy()
	g.scale = 0.25
	for range 4 * 2 * hourFrames {
		g.step(0, 0)
		for _, a := range g.actors() {
			if a.Stuck() >= stuckFrames {
				t.Fatalf("at scale 0.25 a walker got nowhere for %d world frames at (%.0f,%.0f)", a.Stuck(), a.X, a.Y)
			}
		}
	}
}

// D1 (T316): at 3× everything runs three times as fast: a step at scale 3 is three at scale 1, for the whole busy
// hotel: the Clock, the walkers, the courier, the housekeepers, the ghosts and their Shadows, rent and Mood.
func TestFastScale(t *testing.T) {
	busy := func() *Game {
		g, _ := headless(t, 2, 1, 1)
		g.startRun()
		g.DevBusy()
		return g
	}
	a, b := busy(), busy()
	a.scale = 3
	courier, shadow := false, false
	for i := range 4 * hourFrames {
		if i%3 == 0 {
			a.step(0, 0)
		}
		if b.step(0, 0); i%3 == 2 && state(a) != state(b) {
			t.Fatalf("after %d frames at scale 1, %d at scale 3 differ:\n%s\n%s", i+1, (i+1)/3, state(b), state(a))
		}
		courier, shadow = courier || len(b.couriers) > 0, shadow || len(b.areaShades) > 0 || len(b.shades[0]) > 0
	}
	if !courier || !shadow {
		t.Errorf("the run tested no courier (%v) or no Shadow (%v)", courier, shadow)
	}
}

// state: what the world is at this frame, to compare two Runs.
func state(g *Game) string {
	s := fmt.Sprint(g.clock, g.tick, g.wallet, g.mood, g.fog, len(g.parties), len(g.couriers))
	for _, a := range g.actors() {
		s += fmt.Sprint(" ", a.X, a.Y, len(a.Path), a.Fear)
	}
	for _, k := range g.keepers {
		s += fmt.Sprint(" ", k.job)
	}
	for _, list := range append(slices.Clone(g.shades), g.areaShades) {
		for _, sh := range list {
			s += fmt.Sprint(" ", sh.run, sh.x, sh.fade, sh.lvl, sh.essence)
		}
	}
	for _, d := range g.hotel.Doors {
		s += fmt.Sprint(" ", d.Open())
	}
	return s
}
