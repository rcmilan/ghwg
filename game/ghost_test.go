package game

import (
	"fmt"
	"image"
	"slices"
	"testing"

	"ghosthotel/world"
)

// T090 (D143): the hotel runs the same in both dimensions: same seed, same Parties.
func TestDimensionsRunAlike(t *testing.T) {
	state := func(ghost bool) string {
		g, _ := headless(t, 2, 1, 3)
		g.startRun()
		g.ghost = ghost
		g.clock = 14 * hourFrames // peak arrivals
		for range 2 * hourFrames {
			g.step(0, 0)
		}
		out := fmt.Sprintf("money %d walk-ins %d:", int(g.wallet[Money]), g.walkIns)
		for _, p := range g.parties {
			out += fmt.Sprintf(" [room %d state %d %s]", p.Room, p.state, where(p.Guests))
		}
		return out
	}
	real, ghost := state(false), state(true)
	if real != ghost {
		t.Fatalf("the dimensions diverge:\nreal  %s\nghost %s", real, ghost)
	}
	if real == "money 0 walk-ins 0:" {
		t.Fatal("no Walk-in in 2 peak hours: the test proves nothing")
	}
}

// T094 (D158): the Fog hides a settled Party in the real dimension only.
func TestGhostSeesThroughFog(t *testing.T) {
	g, _ := headless(t, 2, 1, 1)
	g.startRun()
	p := g.walkIn()
	p.Routine = homebody
	for i := 0; len(g.fog) == 0 || g.fog[p.Room] < 1; i++ {
		if i > 60*tps {
			t.Fatal("the Room never fogged")
		}
		g.step(0, 0)
	}
	at := image.Pt(int(p.Guests[0].X), int(p.Guests[0].Y))
	if g.fogAt(at) == 0 || slices.Contains(g.view().Guests, at.Div(world.TileSize)) {
		t.Fatal("real: a Guest in a fogged Room is in sight")
	}
	g.ghost = true
	if v := g.view(); g.fogAt(at) != 0 || !slices.Contains(v.Guests, at.Div(world.TileSize)) || !v.Ghost {
		t.Fatal("ghost: the Fog still hides the Guest")
	}
}

// T095: the Pilot flips by keys alone, and asking for the dimension it's in presses nothing.
func TestPilotFlip(t *testing.T) {
	g, _ := headless(t, 2, 1, 7)
	a := g.autopilot(nop{}, 30, 6)
	p := a.pilot
	run(t, g, tps, func() bool { return g.state == StatePlay })
	g.unlock(Mirror) // a covered Mirror flips nothing
	p.Flip(true)
	run(t, g, 60*tps, func() bool { return p.Idle() })
	if !g.ghost {
		t.Fatal("Flip(true) left the hotel real")
	}
	last := p.lastTap
	p.Flip(true)
	run(t, g, 60*tps, func() bool { return p.Idle() })
	if p.lastTap != last || !g.ghost {
		t.Fatal("Flip(true) while ghost pressed a key")
	}
	p.Flip(false)
	run(t, g, 60*tps, func() bool { return p.Idle() })
	if g.ghost {
		t.Fatal("Flip(false) left the hotel ghost")
	}
}
