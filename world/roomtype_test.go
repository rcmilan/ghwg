package world

import (
	"encoding/json"
	"image"
	"io/fs"
	"strings"
	"testing"

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

func TestStandardRoomType(t *testing.T) {
	ts := loadTileset(t)
	types, err := LoadRoomTypes(assets.FS, ts)
	if err != nil {
		t.Fatal(err)
	}
	var std *RoomType
	for i := range types {
		if types[i].Name == "Standard" {
			std = &types[i]
		}
	}
	if std == nil {
		t.Fatalf("no Standard room type in %d types", len(types))
	}
	if std.Rate != 25 || std.DoorCol != 6 {
		t.Errorf("rate %d door col %d, want 25 and 6", std.Rate, std.DoorCol)
	}
	if cell := std.Stay.Div(TileSize); cell != image.Pt(6, 4) {
		t.Errorf("stay cell %v, want (6,4)", cell)
	}
}

// mutate decodes standard.tmj into generic JSON, applies f, and parses the result.
func mutate(t *testing.T, f func(m map[string]any)) error {
	t.Helper()
	data, err := fs.ReadFile(assets.FS, "rooms/standard.tmj")
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatal(err)
	}
	f(m)
	out, _ := json.Marshal(m)
	_, err = ParseRoomType(out, loadTileset(t))
	return err
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
		{"two door columns", "door columns", func(m map[string]any) {
			decor := layer(m, "decor")["data"].([]any)
			decor[7*RoomW+2] = decor[7*RoomW+6] // a second door leaf at col 2
		}},
		{"wrong size", "want 13x9", func(m map[string]any) { m["width"] = 12.0 }},
		{"no rate", `"rate"`, func(m map[string]any) { m["properties"] = m["properties"].([]any)[:1] }}, // keeps "name"
		{"stay on a wall", "not on walkable floor", func(m map[string]any) {
			layer(m, "markers")["objects"].([]any)[0].(map[string]any)["y"] = 8.0 // back face, row 0
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
		t.Fatalf("unmodified standard.tmj: %v", err)
	}
}
