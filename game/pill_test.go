package game

import (
	"os"
	"strings"
	"testing"
)

// Live test (2026-11-15): in the hotel view the ask pills on the right flickered. They were drawn into the pulled-back
// world (g.wide), so the screen-space veils of the Rooms not bought yet (and the Room labels) were drawn over them: a
// pill reaching over a neighbour's veil was cut and dimmed, and as its guest moved it changed from frame to frame.
// A pill is drawn on the screen after them, anchored where the view puts its guest, and kept inside the screen.
func TestPillOnScreenOverVeils(t *testing.T) {
	k, tx, ty := 0.644, 310.0, 28.0
	for _, g := range [][2]float64{{415, 440}, {60, 440}, {500, 20}} { // right side, left side, top
		x, y, hx, hy := balloonAt(g[0], g[1], k, int(-tx), int(-ty), 31, balloonH, balloonScale, screenW, screenH)
		if wx, wy := int(g[0]*k+tx), int(g[1]*k+ty); abs(hx-wx) > 1 || hy < wy-int(float64(charH)*k) || hy > wy {
			t.Errorf("guest %v: head at %d,%d, want it where the view draws him, %d,%d", g, hx, hy, wx, wy)
		}
		if x < 0 || y < 0 || x+31*balloonScale > screenW || y+balloonH*balloonScale > screenH {
			t.Errorf("guest %v: pill at %d,%d runs off the %dx%d screen", g, x, y, screenW, screenH)
		}
	}
	b, err := os.ReadFile("hotelview.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(b)
	veil, pills := strings.Index(src, "fogAlpha * g.viewT"), strings.LastIndex(src, "	pills() ")
	if pills < 0 || veil < 0 || pills < veil {
		t.Errorf("drawView must draw the pills on the screen after the Rooms' veils (veil at %d, pills at %d)", veil, pills)
	}
}
