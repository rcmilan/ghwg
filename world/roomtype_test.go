package world

import (
	"encoding/json"
	"image"
	"io/fs"
	"strings"
	"testing"
	"testing/fstest"

	"ghosthotel/assets"
)

func loadTileset(t *testing.T) *Tileset {
	t.Helper()
	ts, err := LoadTileset(assets.FS, "tiles/hotel.tsj")
	if err != nil {
		t.Fatal(err)
	}
	return ts
}

// D80-D84: one file per tier × bed layout, with the D82 Rates and one Stay Spot per guest it sleeps.
func TestRoomTypeTable(t *testing.T) {
	types, err := LoadRoomTypes(assets.FS, loadTileset(t))
	if err != nil {
		t.Fatal(err)
	}
	want := map[[2]string]int{
		{"Standard", "Single"}: 25, {"Standard", "Twin"}: 40, {"Standard", "Double"}: 40,
		{"Deluxe", "Single"}: 40, {"Deluxe", "Twin"}: 64, {"Deluxe", "Double"}: 64,
	}
	seen := map[[2]string]bool{}
	for _, rt := range types {
		k := [2]string{rt.Name, rt.Beds}
		rate, ok := want[k]
		if !ok || rt.Rate != rate || rt.DoorCol != 4 {
			t.Errorf("%v: rate %d door col %d, want %d and 4", k, rt.Rate, rt.DoorCol, rate)
		}
		if len(rt.Stays) != bedCapacity[rt.Beds] {
			t.Errorf("%v: %d Stay Spots, want %d", k, len(rt.Stays), bedCapacity[rt.Beds])
		}
		for i, p := range rt.Stays { // T140, D167: a bed straight above each Stay Spot, 3 cells tall
			c := p.Div(TileSize)
			want := image.Rectangle{image.Pt(c.X, c.Y-3).Mul(TileSize), image.Pt(c.X+1, c.Y).Mul(TileSize)}
			if i >= len(rt.Lanes) || rt.Lanes[i] != want {
				t.Errorf("%v: Stay Spot %v has bed lanes %v, want %v", k, p, rt.Lanes, want)
			}
		}
		seen[k] = true
	}
	if len(seen) != len(want) {
		t.Errorf("tier × beds combinations %v, want all %d", seen, len(want))
	}
}

// mutate decodes standard_single.tmj into generic JSON, applies f, and parses the result.
func mutate(t *testing.T, f func(m map[string]any)) error {
	t.Helper()
	_, err := ParseRoomType(mutated(t, f), loadTileset(t))
	return err
}

// mutated returns standard_single.tmj with f applied.
func mutated(t *testing.T, f func(m map[string]any)) []byte {
	t.Helper()
	data, err := fs.ReadFile(assets.FS, "rooms/standard_single.tmj")
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatal(err)
	}
	f(m)
	out, _ := json.Marshal(m)
	return out
}

func layer(m map[string]any, name string) map[string]any {
	for _, l := range m["layers"].([]any) {
		if l := l.(map[string]any); l["name"] == name {
			return l
		}
	}
	return nil
}

func TestRoomTypeValidation(t *testing.T) {
	cases := []struct {
		name, want string
		f          func(m map[string]any)
	}{
		{"no stay", `"stay"`, func(m map[string]any) { layer(m, "markers")["objects"] = []any{} }},
		{"no beds", `"beds"`, func(m map[string]any) { m["properties"] = m["properties"].([]any)[:2] }}, // keeps name, rate
		{"a Twin with one stay", "want 2 for Twin", func(m map[string]any) {
			m["properties"].([]any)[2].(map[string]any)["value"] = "Twin"
		}},
		{"two door columns", "door columns", func(m map[string]any) {
			decor := layer(m, "decor")["data"].([]any)
			decor[7*RoomW+2] = decor[7*RoomW+4] // a second door leaf at col 2
		}},
		{"wrong size", "want 9x9", func(m map[string]any) { m["width"] = 12.0 }},
		{"no rate", `"rate"`, func(m map[string]any) { m["properties"] = m["properties"].([]any)[:1] }}, // keeps "name"
		{"stay on a wall", "not on walkable floor", func(m map[string]any) {
			layer(m, "markers")["objects"].([]any)[0].(map[string]any)["y"] = 24.0 // back face bottom, row 1 (the top row is walkable, D66)
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := mutate(t, c.f)
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Fatalf("got %v, want error containing %q", err, c.want)
			}
		})
	}
	if err := mutate(t, func(map[string]any) {}); err != nil {
		t.Fatalf("unmodified standard_single.tmj: %v", err)
	}
}

// D45: doors never move, so every Room Type keeps the same door column.
func TestRoomTypesShareDoorColumn(t *testing.T) {
	moved := mutated(t, func(m map[string]any) {
		decor := layer(m, "decor")["data"].([]any)
		for _, row := range []int{7, 8} {
			decor[row*RoomW+2], decor[row*RoomW+4] = decor[row*RoomW+4], 0.0
		}
	})
	std := mutated(t, func(map[string]any) {})
	fsys := fstest.MapFS{"rooms/a.tmj": {Data: std}, "rooms/b.tmj": {Data: moved}}
	if _, err := LoadRoomTypes(fsys, loadTileset(t)); err == nil || !strings.Contains(err.Error(), "door column") {
		t.Fatalf("got %v, want a door column error", err)
	}
}
