package world

import (
	"math/rand/v2"
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

// D46: each Room draws independently, equal odds, repeats allowed.
func TestDraw(t *testing.T) {
	a, b := RoomType{Name: "A"}, RoomType{Name: "B"}
	rng := rand.New(rand.NewPCG(3, 4))
	seen := make([]map[string]int, 4)
	mixed := 0
	for range 400 {
		next := Draw([]RoomType{a, b}, 4, rng)
		if len(next) != 4 {
			t.Fatalf("drew %d types, want 4", len(next))
		}
		for i, rt := range next {
			if seen[i] == nil {
				seen[i] = map[string]int{}
			}
			seen[i][rt.Name]++
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
		s := r.Stay.Div(TileSize)
		return h.Map.TS.Tiles[h.Map.At(LayerFloor, s.X, s.Y)].Name
	}
	for _, rt := range []RoomType{dlx, std, dlx} {
		next := make([]RoomType, len(h.Rooms))
		for i := range next {
			next[i] = rt
		}
		h.Spin(next)
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
