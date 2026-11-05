package game

import (
	"testing"

	"ghosthotel/world"
)

// D71–D74: a Run starts on Day 1 at 08:00, an hour is 600 frames, and the cinematic pauses the Clock.
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
	g.spin(world.Draw(g.types, len(g.hotel.Rooms), g.rng), nil, false)
	for c := g.clock; g.spinning(); c = g.clock {
		if g.step(0, 0); g.spinning() && g.clock != c {
			t.Fatalf("the clock ran during the Spin cinematic, at tick %d", g.tick)
		}
	}
	c := g.clock
	g.fast = true
	g.step(0, 0)
	if g.clock != c+fastClock {
		t.Errorf("holding T ran the clock %d frames in one step, want %d", g.clock-c, fastClock)
	}
}

// D85: Midnight comes once per day, and with every Room Vacant it Spins them all with the cinematic.
func TestMidnightOncePerDay(t *testing.T) {
	g := mustNew(t)
	g.startRun()
	spins, was := 0, false
	for g.clock < runStart+2*dayFrames {
		g.step(0, 0)
		if g.spinning() && !was {
			spins++
			if hour(g.clock) != 0 || len(g.spinRooms) != len(g.hotel.Rooms) {
				t.Fatalf("a Spin at %s rolling %v, want every Room at 00:00", clockText(g.clock), g.spinRooms)
			}
		}
		was = g.spinning()
	}
	if spins != 2 {
		t.Errorf("%d Spins in 2 days, want 2", spins)
	}
}
