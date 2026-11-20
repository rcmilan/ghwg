package game

import (
	"math"
	"slices"
	"testing"
)

// The spiral: at lift-off it sits on its start point, the angle only grows, the radius only shrinks, and at the
// flash it is on the clock, radius 0.
func TestSpiral(t *testing.T) {
	const x0, y0, cx, cy = 100.0, 400.0, 480.0, 270.0
	for _, start := range []int{reelRace, reelRace + 45, reelRace + liftSpan} {
		x, y, ang, s := spiralAt(x0, y0, cx, cy, start, start)
		if math.Hypot(x-x0, y-y0) > 1e-9 || ang != 0 || s != 1 {
			t.Errorf("start %d: at lift-off (%v,%v) ang %v scale %v", start, x, y, ang, s)
		}
		x, y, _, _ = spiralAt(x0, y0, cx, cy, start, reelFlash)
		if math.Hypot(x-cx, y-cy) > 1e-9 {
			t.Errorf("start %d: at the flash (%v,%v), not the centre", start, x, y)
		}
		prevAng, prevR := -1.0, math.Inf(1)
		for f := start; f <= reelFlash; f++ {
			x, y, ang, _ := spiralAt(x0, y0, cx, cy, start, f)
			if r := math.Hypot(x-cx, y-cy); ang < prevAng || r > prevR+1e-9 {
				t.Fatalf("start %d frame %d: angle %v (was %v), radius %v (was %v)", start, f, ang, prevAng, r, prevR)
			} else {
				prevAng, prevR = ang, r
			}
		}
	}
}

// Furniture is grouped whole; walls, windows and doors don't fly.
func TestGroupTiles(t *testing.T) {
	var ts []vTile
	for r := range 3 { // a bed, 2 wide, 3 tall, its top-left at (5,4)
		for c := range 2 {
			ts = append(ts, vTile{"prop_bed_brown_r" + string(rune('0'+r)) + "c" + string(rune('0'+c)), 2, 5 + c, 4 + r})
		}
	}
	ts = append(ts, vTile{"prop_bed_brown_r0c0", 2, 9, 4}) // another bed of the same kind
	for x := range 3 {                                     // a counter run, and a computer on it, over its own tile
		ts = append(ts, vTile{[]string{"counter_left", "counter_mid", "counter_right"}[x], 2, 10 + x, 8})
	}
	ts = append(ts, vTile{"counter_computer_top", 2, 11, 7}, vTile{"counter_mid", 2, 14, 8}) // the last is apart
	ts = append(ts, vTile{"wall_mid_top", 1, 0, 0}, vTile{"prop_window_front_r0c0", 2, 3, 0}, vTile{"window_curtain_r0c0", 2, 3, 1},
		vTile{"door_plain_top", 2, 4, 0}, vTile{"prop_stairs_r0c0", 2, 6, 0}, vTile{"void", 2, 7, 0}, vTile{"entrance_left_top", 2, 8, 0})
	var sizes []int
	for _, g := range groupTiles(ts) {
		sizes = append(sizes, len(g))
	}
	slices.Sort(sizes)
	if want := []int{1, 1, 4, 6}; !slices.Equal(sizes, want) {
		t.Errorf("object sizes %v, want %v (a bed of 6, a bed of 1, a counter of 3+1, a lone counter)", sizes, want)
	}
}
