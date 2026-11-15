package entities

import "testing"

// walk feeds the animation n frames of movement by (dx,dy) per frame.
func walk(a *Anim, x, y *float64, dx, dy float64, n int) {
	for range n {
		*x += dx
		*y += dy
		a.Update(*x, *y, 1)
	}
}

func TestAnimFacingFromVelocity(t *testing.T) {
	cases := []struct {
		name   string
		dx, dy float64
		want   Facing
	}{
		{"right", 1, 0, FaceRight},
		{"up", 0, -1, FaceUp},
		{"left", -0.6, 0, FaceLeft},
		{"down", 0, 0.6, FaceDown},
		{"mostly down", 0.2, 0.6, FaceDown},
		{"mostly left", -0.6, 0.2, FaceLeft},
	}
	for _, c := range cases {
		a, x, y := NewAnim(), 0.0, 0.0
		a.Update(x, y, 1)
		walk(&a, &x, &y, c.dx, c.dy, 3)
		if a.Facing != c.want || !a.Moving {
			t.Errorf("%s: facing %d moving %v, want facing %d moving", c.name, a.Facing, a.Moving, c.want)
		}
	}
}

func TestAnimDiagonalKeepsFacing(t *testing.T) {
	a, x, y := NewAnim(), 0.0, 0.0
	a.Update(x, y, 1)
	walk(&a, &x, &y, 0, -1, 2) // facing up
	walk(&a, &x, &y, 0.7, -0.7, 10)
	if a.Facing != FaceUp {
		t.Errorf("exact diagonal up-right switched facing to %d, want to keep up", a.Facing)
	}
	walk(&a, &x, &y, -1, 0, 2) // facing left
	walk(&a, &x, &y, 0.7, 0.7, 2)
	if a.Facing != FaceRight {
		t.Errorf("diagonal down-right from facing left: facing %d, want right (horizontal wins ties)", a.Facing)
	}
}

func TestAnimFrames(t *testing.T) {
	a, x, y := NewAnim(), 0.0, 0.0
	a.Update(x, y, 1)
	if a.Moving || a.Facing != FaceDown {
		t.Fatalf("a new actor should idle facing down, got facing %d moving %v", a.Facing, a.Moving)
	}
	walk(&a, &x, &y, -1, 0, 1)
	seen := []int{a.Frame()}
	for range 60 { // one second at 8 fps: 8 frame changes, cycling through 6 frames
		walk(&a, &x, &y, -1, 0, 1)
		if f := a.Frame(); f != seen[len(seen)-1] {
			seen = append(seen, f)
		}
	}
	want := []int{12, 13, 14, 15, 16, 17, 12, 13} // left strip starts at 2*6
	if len(seen) < len(want) {
		t.Fatalf("frames %v, want %v", seen, want)
	}
	for i, f := range want {
		if seen[i] != f {
			t.Fatalf("frames %v, want %v", seen, want)
		}
	}
	a.Update(x, y, 1) // stopped
	if a.Moving || a.Frame() != 12 {
		t.Errorf("after stopping: moving %v frame %d, want idle from the left strip's first frame (12)", a.Moving, a.Frame())
	}
}

// Standing still, the idle cycle plays once, then holds its first frame for a random pause.
func TestAnimIdlePauses(t *testing.T) {
	a, x, y := NewAnim(), 0.0, 0.0
	a.Update(x, y, 1)
	walk(&a, &x, &y, 1, 0, 5)
	a.Update(x, y, 1) // stop
	first := int(FaceRight) * FramesPerFacing
	held, cycled := 0, false
	for f := range idlePauseMax + idleCycle + 1 {
		if a.Frame() != first {
			if f < idlePauseMin {
				t.Fatalf("idle cycle started after %d frames, want a pause of at least %d", f, idlePauseMin)
			}
			cycled = true
			break
		}
		held++
		a.Update(x, y, 1)
	}
	if !cycled {
		t.Fatalf("idle held frame %d for %d frames, want the cycle to play within %d", first, held, idlePauseMax+idleCycle)
	}
	for f := range idleCycle + 1 { // the cycle plays through, then holds again
		a.Update(x, y, 1)
		if a.Frame() == first && f < idleCycle-2*tps/animFPS {
			t.Fatalf("cycle back on the first frame after %d of %d frames", f, idleCycle)
		}
	}
	for range idlePauseMin - 10 { // the hold began a few frames before the cycle loop ended
		if a.Update(x, y, 1); a.Frame() != first {
			t.Fatalf("idle frame %d during the pause, want %d", a.Frame(), first)
		}
	}
}
