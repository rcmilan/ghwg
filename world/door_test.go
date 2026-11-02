package world

import (
	"image"
	"testing"
)

func TestDoorAllows(t *testing.T) {
	room, entrance := Door{Room: 0}, Door{Room: -1}
	const concierge = -1
	cases := []struct {
		name                     string
		door                     Door
		actor                    int
		inside, occupied, allows bool
	}{
		{"own guest, room vacant", room, 0, false, false, true},
		{"own guest, room occupied", room, 0, true, true, true},
		{"another room's guest", room, 2, false, false, false},
		{"concierge, room vacant", room, concierge, false, false, true},
		{"concierge outside an occupied room", room, concierge, false, true, false},
		{"concierge inside an occupied room", room, concierge, true, true, true},
		{"concierge at the Entrance", entrance, concierge, false, true, true},
		{"any guest at the Entrance", entrance, 3, false, false, true},
	}
	for _, c := range cases {
		if got := c.door.Allows(c.actor, c.inside, c.occupied); got != c.allows {
			t.Errorf("%s: allows %v, want %v", c.name, got, c.allows)
		}
	}
}

func TestDoorSolidUnlessFullyOpen(t *testing.T) {
	for slide, solid := range map[int]bool{0: true, DoorSlideFrames / 2: true, DoorSlideFrames - 1: true, DoorSlideFrames: false} {
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
	l := newLayout(Cols, Rows)
	if want := image.Rect(14*TileSize, l.front*TileSize, 16*TileSize, (l.front+2)*TileSize); e.Rect != want || e.Room != -1 {
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
	run := func(frames int, occupied bool, actors ...Actor) {
		for range frames {
			h.UpdateDoors(actors, func(int) bool { return occupied })
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
	run(DoorSlideFrames, true, Actor{outside, -1})
	if d.Open() != 0 {
		t.Errorf("occupied room: opened %.2f for the concierge outside", d.Open())
	}
	run(DoorSlideFrames, true, Actor{inside, -1})
	if d.Solid() {
		t.Errorf("occupied room: not open for the concierge inside (open %.2f)", d.Open())
	}

	reset()
	run(DoorSlideFrames, true, Actor{outside, 0})
	if d.Solid() {
		t.Errorf("not open for its own guest (open %.2f)", d.Open())
	}
	if run(DoorSlideFrames, true, Actor{doorway, 2}); d.Solid() {
		t.Error("closed on someone standing in the doorway")
	}
	if run(DoorSlideFrames, true); d.Open() != 0 {
		t.Errorf("still open %.2f with nobody near", d.Open())
	}
}
