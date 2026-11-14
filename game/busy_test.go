package game

import (
	"image"
	"testing"
)

// The busy hotel: a full 3x3 with everyone walking at once. Nobody stands stuck, and walkers are seldom
// stopped by someone ahead.
func TestBusyHotel(t *testing.T) {
	g, _ := headless(t, 2, 1, 1)
	g.startRun()
	g.DevBusy()
	if c, r := g.hotel.Grid(); c != maxCols || r != maxRows || len(g.keepers) != keeperMax {
		t.Fatalf("the busy hotel is %dx%d with %d housekeepers", c, r, len(g.keepers))
	}
	worst, where, most := 0, image.Point{}, 0
	for range 2 * dayFrames {
		g.step(0, 0)
		most = max(most, len(g.actors()))
		for _, a := range g.actors() {
			if a.Stuck() > worst {
				worst, where = a.Stuck(), image.Pt(int(a.X), int(a.Y))
			}
		}
	}
	share := float64(g.bumps) / float64(max(g.paces, 1))
	t.Logf("up to %d on the map; stopped by someone ahead %.2f%% of %d paces; the longest anyone got nowhere: %d frames, at %v; %d serves, %d expired", most, 100*share, g.paces, worst, where, g.serves, g.expired)
	if worst >= stuckFrames {
		t.Errorf("someone got nowhere for %d frames at %v, want under %d", worst, where, stuckFrames)
	}
	if share > busyBumps {
		t.Errorf("walkers were stopped by someone ahead %.2f%% of the time, want at most %.2f%%", 100*share, 100*busyBumps)
	}
}

const busyBumps = 0.005 // measured 0.0005 (2026-11-14); before walkers steered ahead, 0.002
