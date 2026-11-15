package world

import (
	"image"
	"strings"
	"testing"
)

func TestDoorAllows(t *testing.T) { // K1 (2026-11-11): the staff open any Room's door, a Guest only its own
	room, entrance := Door{Room: 0}, Door{Room: -1}
	const staff = -1
	cases := []struct {
		name   string
		door   Door
		actor  int
		allows bool
	}{
		{"own guest", room, 0, true},
		{"another room's guest", room, 2, false},
		{"the staff at a Room", room, staff, true},
		{"the staff at the Entrance", entrance, staff, true},
		{"any guest at the Entrance", entrance, 3, true},
	}
	for _, c := range cases {
		if got := c.door.Allows(c.actor); got != c.allows {
			t.Errorf("%s: allows %v, want %v", c.name, got, c.allows)
		}
	}
}

func TestDoorSolidUnlessFullyOpen(t *testing.T) {
	for slide, solid := range map[float64]bool{0: true, DoorSlideFrames / 2: true, DoorSlideFrames - 1: true, DoorSlideFrames: false} {
		d := Door{slide: slide}
		if d.Solid() != solid {
			t.Errorf("open %.2f: solid %v, want %v", d.Open(), d.Solid(), solid)
		}
	}
}

func TestHotelDoors(t *testing.T) {
	h := buildHotel(t)
	if len(h.Doors) != len(h.Rooms)+1 {
		t.Fatalf("%d doors, want one per Room plus the Entrance", len(h.Doors))
	}
	for i, r := range h.Rooms {
		top := r.Door.Mul(TileSize)
		if want := (image.Rectangle{top, top.Add(image.Pt(TileSize, 2*TileSize))}); h.Doors[i].Rect != want || h.Doors[i].Room != i {
			t.Errorf("room %s door %+v, want rect %v", r.Number, h.Doors[i], want)
		}
	}
	e := h.Doors[len(h.Rooms)]
	l := newLayout(2, 2)
	if want := image.Rect(l.entrance*TileSize, l.front*TileSize, (l.entrance+2)*TileSize, (l.front+2)*TileSize); e.Rect != want || e.Room != -1 {
		t.Errorf("entrance %+v, want rect %v", e, want)
	}
	if !h.DoorBlocks(e.Rect.Inset(4)) {
		t.Error("a shut Entrance does not block")
	}
}

// foot box (10×6) centered on p
func footAt(p image.Point) image.Rectangle { return image.Rect(p.X-5, p.Y-3, p.X+5, p.Y+3) }

func TestUpdateDoors(t *testing.T) {
	h := buildHotel(t)
	d := &h.Doors[0]
	mid := (d.Rect.Min.X + d.Rect.Max.X) / 2
	outside := footAt(image.Pt(mid, d.Rect.Max.Y+6)) // corridor, just below the door
	inside := footAt(image.Pt(mid, d.Rect.Min.Y-6))  // room floor, just above the door
	doorway := footAt(image.Pt(mid, d.Rect.Min.Y+20))
	run := func(frames int, boarded bool, actors ...Actor) {
		for range frames {
			h.UpdateDoors(actors, func(int) bool { return boarded }, 1)
		}
	}
	reset := func() {
		for i := range h.Doors {
			h.Doors[i].slide = 0
		}
	}

	run(DoorSlideFrames-1, false, Actor{outside, -1})
	if !d.Solid() {
		t.Fatal("opened faster than DoorSlideFrames")
	}
	run(1, false, Actor{outside, -1})
	if d.Solid() || h.DoorBlocks(doorway) {
		t.Fatalf("vacant room: not open for the concierge after %d frames (open %.2f)", DoorSlideFrames, d.Open())
	}

	reset()
	run(DoorSlideFrames, true, Actor{outside, -1}, Actor{outside, 0}, Actor{inside, -1})
	if d.Open() != 0 {
		t.Errorf("a boarded Room opened %.2f", d.Open())
	}
	run(DoorSlideFrames, false, Actor{inside, -1})
	if d.Solid() {
		t.Errorf("not open for the staff inside (open %.2f)", d.Open())
	}

	reset()
	run(DoorSlideFrames, false, Actor{outside, 2})
	if d.Open() != 0 {
		t.Errorf("opened %.2f for another Room's guest", d.Open())
	}
	run(DoorSlideFrames, false, Actor{outside, 0})
	if d.Solid() {
		t.Errorf("not open for its own guest (open %.2f)", d.Open())
	}
	if run(DoorSlideFrames, false, Actor{doorway, 2}); d.Solid() {
		t.Error("closed on someone standing in the doorway")
	}
	if run(DoorSlideFrames, false); d.Open() != 0 {
		t.Errorf("still open %.2f with nobody near", d.Open())
	}
}

// 08 F0: a Room's Mat is its doormat, the door's Spot inside it; the Plate is the mat's cell west of the door.
func TestMat(t *testing.T) {
	forGrids(t, func(t *testing.T, h *Hotel, l layout) {
		for _, r := range h.Rooms {
			m := r.Mat()
			if !r.Spot().In(m) || !r.Plate().In(m) || r.Plate().Overlaps(r.Spot()) {
				t.Errorf("room %s: mat %v, spot %v, plate %v", r.Number, m, r.Spot(), r.Plate())
			}
			for y := m.Min.Y / TileSize; y < m.Max.Y/TileSize; y++ {
				for x := m.Min.X / TileSize; x < m.Max.X/TileSize; x++ {
					if n := h.Map.TS.Tiles[h.Map.At(LayerDecor, x, y)].Name; !strings.HasPrefix(n, "prop_doormat") {
						t.Errorf("room %s: %q at (%d,%d), want the doormat", r.Number, n, x, y)
					}
				}
			}
		}
	})
}
