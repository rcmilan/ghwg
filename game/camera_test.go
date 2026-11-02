package game

import "testing"

func TestCameraClampsToMap(t *testing.T) {
	const mw, mh = 512, 608
	cases := []struct {
		name           string
		tx, ty, wx, wy float64
	}{
		{"top-left corner", 0, 0, 0, 0},
		{"bottom-right corner", mw, mh, mw - viewW, mh - viewH},
		{"middle", 256, 300, 256 - viewW/2, 300 - viewH/2},
	}
	for _, c := range cases {
		var cam Camera
		cam.Snap(c.tx, c.ty, mw, mh)
		if cam.X != c.wx || cam.Y != c.wy {
			t.Errorf("%s: camera at (%v,%v), want (%v,%v)", c.name, cam.X, cam.Y, c.wx, c.wy)
		}
	}
}

func TestCameraConvergesSmoothly(t *testing.T) {
	var cam Camera
	cam.Snap(0, 0, 512, 608)
	prev := cam.X
	for frame := range 60 {
		cam.Follow(400, 300, 512, 608)
		if step := cam.X - prev; step > 0.15*(512-viewW)+1e-9 {
			t.Fatalf("frame %d: jumped %.1fpx, want an eased step", frame, step)
		}
		prev = cam.X
	}
	if x, y := cam.Origin(400, 300, 512, 608); x != (512-viewW)*zoom || y != (300-viewH/2)*zoom {
		t.Errorf("after 60 frames at pixel (%d,%d), want (%d,%d)", x, y, (512-viewW)*zoom, (300-viewH/2)*zoom)
	}
}

// Walking diagonally once the camera has settled, the map must scroll every frame by an even 2–3 screen
// px — never stall a frame then jump a whole world px (the old 320×180 view stepped 0 or 3).
func TestCameraDiagonalScrollIsSmooth(t *testing.T) {
	const mw, mh, v = 512, 608, 0.7071
	px, py := 150.0, 200.0
	var cam Camera
	cam.Snap(px, py, mw, mh)
	var prevX, prevY int
	for frame := range 220 {
		px += v
		py += v
		cam.Follow(px, py, mw, mh)
		ox, oy := cam.Origin(px, py, mw, mh)
		if frame > 120 { // camera lag settled
			if dx, dy := ox-prevX, oy-prevY; dx < 2 || dx > 3 || dy < 2 || dy > 3 {
				t.Fatalf("frame %d: map scrolled (%d,%d) screen px, want 2–3 on both axes", frame, dx, dy)
			}
		}
		prevX, prevY = ox, oy
	}
}

// Walking diagonally at a sub-pixel speed, the player's on-screen position must ease in one direction
// and never jitter back and forth (the old independent rounding flickered 1px every few frames).
func TestCameraNoDiagonalFlicker(t *testing.T) {
	const mw, mh = 512, 608
	px, py := 200.0, 300.0
	var cam Camera
	cam.Snap(px, py, mw, mh)
	prevX, prevY, lastDX, lastDY := -1, -1, 0, 0
	for frame := range 200 {
		px += 0.7071
		py += 0.7071
		cam.Follow(px, py, mw, mh)
		ox, oy := cam.Origin(px, py, mw, mh)
		sx, sy := screenPx(px)-ox, screenPx(py)-oy // where the player is drawn
		if frame > 0 {
			dx, dy := sx-prevX, sy-prevY
			if dx*lastDX < 0 || dy*lastDY < 0 {
				t.Fatalf("frame %d: on-screen player reversed (%d,%d) after (%d,%d)", frame, dx, dy, lastDX, lastDY)
			}
			if dx != 0 {
				lastDX = dx
			}
			if dy != 0 {
				lastDY = dy
			}
		}
		prevX, prevY = sx, sy
	}
}
