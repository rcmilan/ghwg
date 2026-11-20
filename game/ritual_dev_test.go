package game

import "testing"

// W3.5 T369: -ritual loops a Father's ritual: the dirt comes back and he does it again; kpace only paces.
func TestDevRitual(t *testing.T) {
	for _, who := range []string{"k", "m", "kpace"} {
		g, _ := headless(t, 3, 3, 1)
		g.openAtStart = false
		g.startRun()
		g.DevRitual(who)
		frames := 3000
		if who == "kpace" {
			frames = 1000 // before the Room's first guest settles and a ghost may haunt it
		}
		bursts, last := 0, phase(-1)
		for range frames {
			g.step(0, 0)
			if p := g.priests[0].phase; p != last && p == rBurst {
				bursts++
			} else if p == rChant && g.spiritDirt(g.priests[0].RoomIndex) == 0 {
				t.Fatalf("%s: chanting over a clean Room", who)
			}
			last = g.priests[0].phase
		}
		if want := who != "kpace"; (bursts >= 2) != want || (who == "kpace" && bursts != 0) {
			t.Fatalf("%s: %d rituals, want a loop (2+) for k and m, none for kpace", who, bursts)
		}
	}
}
