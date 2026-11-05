package world

import (
	"math/rand/v2"
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

// D46: each Room draws independently, equal odds, repeats allowed. D65: odds are per type name, not per variant.
func TestDraw(t *testing.T) {
	a, a2, b := RoomType{Name: "A"}, RoomType{Name: "A", Rate: 2}, RoomType{Name: "B"}
	rng := rand.New(rand.NewPCG(3, 4))
	seen := make([]map[string]int, 4)
	mixed, variant := 0, 0
	for range 400 {
		next := Draw([]RoomType{a, a2, b}, 4, rng)
		if len(next) != 4 {
			t.Fatalf("drew %d types, want 4", len(next))
		}
		for i, rt := range next {
			if seen[i] == nil {
				seen[i] = map[string]int{}
			}
			seen[i][rt.Name]++
			if rt.Rate == 2 {
				variant++
			}
		}
		if next[0].Name != next[1].Name {
			mixed++
		}
	}
	for i, s := range seen {
		if s["A"] < 150 || s["B"] < 150 {
			t.Errorf("room %d drew %v over 400 Spins, want roughly even odds", i, s)
		}
	}
	if variant < 300 || variant > 500 { // ~400: half of A's ~800 draws
		t.Errorf("A's second variant drawn %d times over 1600 draws, want ~400", variant)
	}
	if mixed < 150 {
		t.Errorf("rooms 0 and 1 differed in %d of 400 Spins: draws are not independent", mixed)
	}
}

// D47: a Spin is given each Room's next type and re-stamps the Rooms; doors, numbers and the hotel's
// outer side lines stay put (D45).
func TestSpin(t *testing.T) {
	h := buildHotel(t)
	std, dlx := typeNamed(t, "Standard"), typeNamed(t, "Deluxe")
	doors := append([]Door(nil), h.Doors...)
	floorAt := func(r Room) string {
		s := r.Stays[0].Div(TileSize)
		return h.Map.TS.Tiles[h.Map.At(LayerFloor, s.X, s.Y)].Name
	}
	for _, rt := range []RoomType{dlx, std, dlx} {
		next := make([]RoomType, len(h.Rooms))
		for i := range next {
			next[i] = rt
		}
		before := floorAt(h.Rooms[0])
		h.Spin(next)
		if h.Rooms[0].Type.Name != rt.Name || floorAt(h.Rooms[0]) != before {
			t.Errorf("Spin to %s: type %s, floor %s; want the type now, the cells at Land (D49)", rt.Name, h.Rooms[0].Type.Name, floorAt(h.Rooms[0]))
		}
		for i := range h.Rooms {
			h.Land(i)
		}
		want := map[string]string{"Standard": "floor_clay", "Deluxe": "floor_herringbone"}[rt.Name]
		for i, r := range h.Rooms {
			if r.Type.Name != rt.Name || floorAt(r) != want {
				t.Errorf("room %s after a %s Spin: type %s, floor %s", r.Number, rt.Name, r.Type.Name, floorAt(r))
			}
			if h.Doors[i] != doors[i] {
				t.Errorf("room %s door moved: %+v -> %+v", r.Number, doors[i], h.Doors[i])
			}
		}
		if n := h.Map.TS.Tiles[h.Map.At(LayerDecor, 1, 5)].Name; n != "border_west" {
			t.Errorf("hotel west line at (1,5) is %q after a Spin, want border_west", n)
		}
	}
}

// D48: a Room is one whole: Land stamps every cell of its footprint from its type, including the
// columns under the hotel's outer side lines, which alone stay (and stay still while it rolls).
func TestLandWholeRoom(t *testing.T) {
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
		h.Spin([]RoomType{rt, rt})
		for i := range h.Rooms {
			h.Land(i)
		}
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

// D84, T029: the draw goes tier → beds → variant, equal odds at each step, however many files each has.
func TestDrawTierBedsVariant(t *testing.T) {
	types := []RoomType{
		{Name: "A", Beds: "Single"}, {Name: "A", Beds: "Twin"}, {Name: "A", Beds: "Twin", Rate: 2}, {Name: "A", Beds: "Double"},
		{Name: "B", Beds: "Single"},
	}
	key := func(rt RoomType) [3]any { return [3]any{rt.Name, rt.Beds, rt.Rate} }
	n := map[[3]any]int{}
	for _, rt := range Draw(types, 12000, rand.New(rand.NewPCG(5, 6))) {
		n[key(rt)]++
	}
	for _, c := range []struct {
		rt   RoomType
		want int // of 12000: B 1/2; A's Single and Double 1/6; each Twin variant 1/12
	}{{types[4], 6000}, {types[0], 2000}, {types[3], 2000}, {types[1], 1000}, {types[2], 1000}} {
		if d := n[key(c.rt)] - c.want; d < -c.want/8 || d > c.want/8 {
			t.Errorf("%v drawn %d times of 12000, want ~%d", key(c.rt), n[key(c.rt)], c.want)
		}
	}
}
