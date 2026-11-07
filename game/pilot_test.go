package game

import (
	"fmt"
	"testing"
)

type nop struct{}

func (nop) Act(View, *Pilot) {}

// run steps the autopiloted Game until done or the frame budget runs out.
func run(t *testing.T, g *Game, frames int, done func() bool) {
	t.Helper()
	for range frames {
		if g.Update(); done() {
			return
		}
	}
	t.Fatalf("not done after %d frames, at %s", frames, clockText(g.clock))
}

// T053: the Pilot walks, opens the Terminal and closes it by keys alone, at the KeyGap pace.
func TestPilot(t *testing.T) {
	for _, grid := range [][2]int{{2, 1}, {3, 3}} {
		t.Run(fmt.Sprintf("%dx%d", grid[0], grid[1]), func(t *testing.T) {
			g, _ := headless(t, grid[0], grid[1], 7)
			a := g.autopilot(nop{}, 30, 6)
			p := a.pilot
			run(t, g, tps, func() bool { return g.state == StatePlay })
			p.GoTo(g.hotel.Archway) // walk away first, so OpenTerminal has to walk back
			p.OpenTerminal()
			run(t, g, 60*tps, func() bool { return g.view().Terminal.Open })
			last, minGap := p.lastTap, 1<<30
			p.Close()
			run(t, g, 60*tps, func() bool {
				if p.lastTap != last {
					minGap, last = min(minGap, p.lastTap-last), p.lastTap
				}
				return p.Idle()
			})
			if g.view().Terminal.Open {
				t.Fatal("the Terminal is still open after Close")
			}
			if minGap < 6 {
				t.Fatalf("two taps %d frames apart, want KeyGap 6 at least", minGap)
			}
		})
	}
}
