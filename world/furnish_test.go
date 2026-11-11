package world

import (
	"strings"
	"testing"

	"ghosthotel/assets"
)

func typeNamed(t *testing.T, name string) RoomType {
	t.Helper()
	types, err := LoadRoomTypes(assets.FS, loadTileset(t))
	if err != nil {
		t.Fatal(err)
	}
	for _, rt := range types {
		if rt.Name == name {
			return rt
		}
	}
	t.Fatalf("no Room Type %q", name)
	return RoomType{}
}

// Furnish re-stamps the Rooms with the types given; doors, numbers and the hotel's outer side lines stay
// put (D45).
func TestFurnish(t *testing.T) {
	h := buildHotel(t)
	std, dlx := typeNamed(t, "Standard"), typeNamed(t, "Deluxe")
	doors := append([]Door(nil), h.Doors...)
	for _, rt := range []RoomType{dlx, std, dlx} {
		next := make([]RoomType, len(h.Rooms))
		for i := range next {
			next[i] = rt
		}
		h.Furnish(next)
		want := map[string]string{"Standard": "floor_clay", "Deluxe": "floor_herringbone"}[rt.Name]
		for i, r := range h.Rooms {
			s := r.Stays[0].Div(TileSize)
			if floor := h.Map.TS.Tiles[h.Map.At(LayerFloor, s.X, s.Y)].Name; r.Type.Name != rt.Name || floor != want {
				t.Errorf("room %s furnished %s: type %s, floor %s", r.Number, rt.Name, r.Type.Name, floor)
			}
			if h.Doors[i] != doors[i] {
				t.Errorf("room %s door moved: %+v -> %+v", r.Number, doors[i], h.Doors[i])
			}
		}
		if n := h.Map.TS.Tiles[h.Map.At(LayerDecor, 1, 5)].Name; n != "border_west" {
			t.Errorf("hotel west line at (1,5) is %q after Furnish, want border_west", n)
		}
	}
}

// D48: a Room is one whole: Furnish stamps every cell of its footprint from its type, except the
// columns under the hotel's outer side lines, which alone stay.
func TestFurnishWholeRoom(t *testing.T) {
	forGrids(t, func(t *testing.T, h *Hotel, l layout) {
		for _, r := range h.Rooms {
			for layer, cells := range r.Type.Tiles {
				for c, want := range cells {
					x, y := r.Origin.X+c%RoomW, r.Origin.Y+c/RoomW
					got := h.Map.At(layer, x, y)
					if layer == LayerDecor && h.OuterLine(x) {
						if n := h.Map.TS.Tiles[got].Name; !strings.HasPrefix(n, "border_") {
							t.Errorf("room %s (%d,%d): outer side line is %q", r.Number, x, y, n)
						}
					} else if got != want {
						t.Errorf("room %s (%d,%d) layer %d: tile %d, want its type's %d", r.Number, x, y, layer, got, want)
					}
				}
			}
			if h.OuterLine(r.Origin.X+1) || h.OuterLine(r.Origin.X+RoomW-2) {
				t.Errorf("room %s: an outer side line runs inside it, not on its edge", r.Number)
			}
		}
	})
}

// D65, T028: every variant keeps every Stay Spot reachable through its own door, and its furniture blocks.
func TestVariantsReachable(t *testing.T) {
	types, err := LoadRoomTypes(assets.FS, loadTileset(t))
	if err != nil {
		t.Fatal(err)
	}
	h := buildGrid(t, 2, 1)
	solid := 0
	for _, rt := range types {
		h.Furnish([]RoomType{rt, rt})
		for i, r := range h.Rooms {
			for _, s := range r.Stays {
				if h.Path(h.Spawn, s, i) == nil {
					t.Errorf("%s variant %v: room %s's Stay Spot %v is unreachable", rt.Name, rt.Stays, r.Number, s)
				}
			}
		}
		for _, id := range rt.Tiles[LayerProps] {
			if len(h.Map.TS.Tiles[id].Solid) > 0 {
				solid++
			}
		}
	}
	if len(types) < 6 || solid == 0 {
		t.Errorf("%d room files with %d solid prop cells, want 6+ variants with solid furniture", len(types), solid)
	}
}

// T276, T278 (K1): every Room opens as the Standard Single, on any grid. Land stamps one Room alone; with
// mirror it hangs the in-Room Mirror on the back wall (the type's own wall mirror if it has one) and marks
// the cells below it; without, no Room shows a mirror nor has a Guest look in one.
func TestLand(t *testing.T) {
	types, err := LoadRoomTypes(assets.FS, loadTileset(t))
	if err != nil {
		t.Fatal(err)
	}
	forGrids(t, func(t *testing.T, h *Hotel, l layout) {
		for _, r := range h.Rooms {
			if r.Type.Name != "Standard" || r.Type.Beds != "Single" || !r.Mirror.Empty() {
				t.Errorf("room %s opens as %s %s, mirror %v; want a Standard Single with none", r.Number, r.Type.Name, r.Type.Beds, r.Mirror)
			}
		}
	})
	h := buildGrid(t, 2, 1)
	mirrors := func(i int) (n int) {
		r := h.Rooms[i]
		for l := range NumLayers {
			for c := range RoomW * RoomH {
				if strings.HasPrefix(h.Map.TS.Tiles[h.Map.At(l, r.Origin.X+c%RoomW, r.Origin.Y+c/RoomW)].Name, mirrorTile) {
					n++
				}
			}
		}
		return n
	}
	looks := func(i int) bool {
		for _, s := range h.Rooms[i].Spots {
			if s.Verb == "look in" {
				return true
			}
		}
		return false
	}
	for _, beds := range []string{"Single", "Twin", "Double"} {
		rt, ok := TypeOf(types, beds)
		if !ok {
			t.Fatalf("no Standard %s", beds)
		}
		before := h.Rooms[1]
		h.Land(0, rt, false)
		if r := h.Rooms[0]; r.Type.Beds != beds || len(r.Stays) != bedCapacity[beds] || mirrors(0) != 0 || looks(0) || !r.Mirror.Empty() {
			t.Errorf("%s landed plain: beds %s, %d Stay Spots, %d mirror tiles, look-in %v, spot %v", beds, r.Type.Beds, len(r.Stays), mirrors(0), looks(0), r.Mirror)
		}
		h.Land(0, rt, true)
		r := h.Rooms[0]
		if mirrors(0) != 2 || r.Mirror.Empty() || !r.Mirror.In(r.Shell()) {
			t.Fatalf("%s landed mirrored: %d mirror tiles, spot %v", beds, mirrors(0), r.Mirror)
		}
		mid := r.Mirror.Min.Add(r.Mirror.Max).Div(2).Div(TileSize)
		if h.Map.Solid(mid.X, mid.Y) || h.Path(h.Spawn, mid.Mul(TileSize), 0) == nil {
			t.Errorf("%s: the cell %v under the Mirror can't be stood on", beds, mid)
		}
		if n := h.Map.TS.Tiles[h.Map.At(LayerProps, mid.X, mid.Y-1)].Name; n != mirrorTile+"r1c0" || !h.Map.Wall(mid.X, mid.Y-1) {
			t.Errorf("%s: over the spot is %q, want the Mirror on the wall", beds, n)
		}
		if after := h.Rooms[1]; after.Type.Beds != before.Type.Beds || mirrors(1) != 0 {
			t.Errorf("%s: landing room 0 changed room 1", beds)
		}
	}
}
