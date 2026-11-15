package world

import (
	"image"
	"strings"
	"testing"

	"ghosthotel/assets"
)

// Corner vanish (F0, 2026-11-15): the top row of a Room's back face, its corners included, was walkable
// and inside its Shell, so routes along the Corridor above it cut through it: fogged out and drawn under
// the wall. No route between Corridor spots may enter a Room.
func TestCorridorRoutesStayOutOfRooms(t *testing.T) {
	forGrids(t, func(t *testing.T, h *Hotel, l layout) {
		mats := []image.Point{h.Archway}
		for _, r := range h.Rooms {
			mats = append(mats, cellCenter(r.Door.X, r.Door.Y+2), cellCenter(r.Door.X-1, r.Door.Y+3))
		}
		for _, a := range mats {
			for _, b := range mats {
				for _, p := range h.Path(a, b, -1) {
					for _, r := range h.Rooms {
						if p.In(r.Shell()) {
							t.Fatalf("route %v -> %v enters Room %s at its cell %v", a, b, r.Number, h.Map.Cell(p).Sub(r.Origin))
						}
					}
				}
			}
		}
	})
}

// Room mirror placement (F0, 2026-11-15): the Room's Mirror replaces the prop it hangs over, whole: a Deluxe
// Room's television goes, and its "watch TV" spot becomes the Mirror's.
func TestRoomMirrorReplacesProp(t *testing.T) {
	h := buildHotel(t)
	types, _ := LoadRoomTypes(assets.FS, h.Map.TS)
	tv, ok := TypeIn(types, "Deluxe", "Double")
	if !ok {
		t.Fatal("no Deluxe Double")
	}
	h.Land(0, tv, true)
	r := h.Rooms[0]
	for y := r.Origin.Y; y < r.Origin.Y+RoomH; y++ {
		for x := r.Origin.X; x < r.Origin.X+RoomW; x++ {
			if n := h.Map.TS.Tiles[h.Map.At(LayerProps, x, y)].Name; strings.HasPrefix(n, "prop_tv_") {
				t.Errorf("a television tile %s at (%d,%d) is left under the Mirror", n, x-r.Origin.X, y-r.Origin.Y)
			}
		}
	}
	verbs := map[string]bool{}
	for _, s := range r.Spots {
		verbs[s.Verb] = true
	}
	if verbs["watch TV"] || !verbs["look in"] {
		t.Errorf("spots %v: want the television's turned to the Mirror's", r.Spots)
	}
}

// Decor (F0): potted plants stand at hand-placed spots, solid, never on a route between the hotel's anchors.
func TestPlantsOutOfTheWay(t *testing.T) {
	forGrids(t, func(t *testing.T, h *Hotel, l layout) {
		if len(h.Plants) == 0 {
			t.Fatal("no plants")
		}
		stops := []image.Point{h.Spawn, h.Archway, h.ConciergeStart, h.Mirror, h.Stock.Min.Add(image.Pt(8, 24))}
		for _, r := range h.Rooms {
			stops = append(stops, cellCenter(r.Door.X, r.Door.Y+2), r.Stays[0])
		}
		for _, p := range h.Plants {
			c := h.Map.Cell(p.At.Sub(image.Pt(0, 1)))
			if !h.Map.Solid(c.X, c.Y) {
				t.Errorf("plant at %v is not solid", c)
			}
			delete(h.Map.Block, c)
			h.Map.Rev++ // where would the routes go without it?
			for _, a := range stops {
				for _, b := range stops {
					for _, w := range h.Path(a, b, -1) {
						if h.Map.Cell(w) == c {
							t.Errorf("plant at %v stands on the route %v -> %v", c, a, b)
						}
					}
				}
			}
			h.Map.Block[c] = true
			h.Map.Rev++
		}
	})
}

// Lighting (F0): every window, the Rooms' and the front face's, lights the floor below it at night.
func TestWindowLights(t *testing.T) {
	h := buildHotel(t)
	if want := 2*len(h.Rooms) + 4; len(h.Windows) != want {
		t.Errorf("%d window lights, want %d", len(h.Windows), want)
	}
	for _, w := range h.Windows {
		if c := h.Map.Cell(w); h.Map.Wall(c.X, c.Y) {
			t.Errorf("window light %v falls on a wall", w)
		}
	}
}
