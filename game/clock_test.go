package game

import "testing"

// D71–D74: a Run starts on Day 1 at 08:00, an hour is 600 frames, and T runs it ×10.
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
	if g.clock != c+fastClock {
		t.Errorf("holding T ran the clock %d frames in one step, want %d", g.clock-c, fastClock)
	}
}
