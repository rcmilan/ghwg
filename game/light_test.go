package game

import (
	"math"
	"testing"
)

// D94, T037: the darkness tint is clear at noon, warm at dusk, deep blue at night and pink at dawn,
// and it never jumps, midnight included.
func TestTint(t *testing.T) {
	at := func(h, m int) [4]float64 { return tint(h*hourFrames + m*minuteFrames) }
	if c := at(12, 0); c[3] != 0 {
		t.Errorf("noon: %v, want clear", c)
	}
	if c := at(19, 0); c[3] == 0 || c[0] <= c[2] {
		t.Errorf("19:00: %v, want a warm tint", c)
	}
	if c := at(2, 0); c[3] < 0.6 || c[2] <= c[0] {
		t.Errorf("02:00: %v, want deep blue", c)
	}
	if c := at(6, 0); c[3] == 0 || c[0] <= c[1] || c[2] <= c[1] {
		t.Errorf("06:00: %v, want pink", c)
	}
	for h, want := range map[int]float64{23: 1, 0: 1, 3: 0, 7: 1, 12: 1} {
		if l := lobbyLit(h * hourFrames); l != want {
			t.Errorf("%02d:00: the Lobby is %.0f%% lit, want %.0f%%", h, l*100, want*100)
		}
	}
	for c := range dayFrames + minuteFrames { // one step past midnight: the wrap is continuous too
		a, b := tint(c), tint(c+minuteFrames)
		if math.Abs(lobbyLit(c)-lobbyLit(c+minuteFrames)) > 0.1 {
			t.Fatalf("frame %d: the Lobby light jumps", c)
		}
		for i := range a {
			if math.Abs(a[i]-b[i]) > 0.02 {
				t.Fatalf("frame %d: the tint jumps from %v to %v in a game minute", c, a, b)
			}
		}
	}
}
