package game

import (
	"image"
	"math"
	"testing"

	"ghosthotel/entities"
)

// Levitation (F0): a ghost floats about 3 px up, drifting 1-2 px up and down every ~2 s.
func TestHover(t *testing.T) {
	lo, hi := math.Inf(1), math.Inf(-1)
	for tick := range 4 * tps {
		h := hover(tick, "Amelia#7")
		lo, hi = min(lo, h), max(hi, h)
		if math.Abs(h-hover(tick+2*tps, "Amelia#7")) > 1e-9 {
			t.Fatalf("tick %d: %.2f, then %.2f two seconds later: want a ~2 s beat", tick, h, hover(tick+2*tps, "Amelia#7"))
		}
	}
	if lo < 2 || hi > 4 || hi-lo < 1 {
		t.Errorf("floats %.2f..%.2f px up, want around 3 with a 1-2 px drift", lo, hi)
	}
}

// Lighting (F0): at night the windows and the kitchen's own lights shine, and a lantern lights the floor
// ahead of the Concierge, the way he faces.
func TestNightLights(t *testing.T) {
	g, _ := headless(t, 2, 1, 1)
	g.startRun()
	g.player.Anim.Facing = entities.FaceUp
	var kitchen, windows, lantern int
	for _, l := range g.lights() {
		switch {
		case l.img == lanternImg && l.at == image.Pt(int(g.player.X), int(g.player.Y)):
			lantern++
			if l.turn != -math.Pi/2 {
				t.Errorf("the lantern turns %.2f facing up, want -pi/2", l.turn)
			}
		case l.at.In(g.hotel.Kitchen):
			kitchen++
		}
		for _, w := range g.hotel.Windows {
			if l.at == w {
				windows++
			}
		}
	}
	if kitchen == 0 || windows != len(g.hotel.Windows) || lantern != 1 {
		t.Errorf("%d kitchen lights, %d of %d windows, %d lanterns on the Concierge", kitchen, windows, len(g.hotel.Windows), lantern)
	}
}
