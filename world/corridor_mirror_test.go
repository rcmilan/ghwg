package world

import (
	"image"
	"strings"
	"testing"

	"ghosthotel/assets"
)

// 08 B1: the Lobby Mirror's spot and a Room mirror's touch only the prop: one cell below it.
func TestMirrorSpotsAreTight(t *testing.T) {
	h := buildHotel(t)
	if h.MirrorSpot.Dx() != TileSize || h.MirrorSpot.Dy() != TileSize || !h.Mirror.In(h.MirrorSpot) {
		t.Errorf("the Lobby Mirror's spot %v: want the one cell under it, with %v in it", h.MirrorSpot, h.Mirror)
	}
	types, _ := LoadRoomTypes(assets.FS, h.Map.TS)
	h.Land(0, types[0], true)
	if m := h.Rooms[0].Mirror; m.Dx() != TileSize || m.Dy() != TileSize {
		t.Errorf("a Room mirror's zone %v: want one cell", m)
	}
}

func decorNames(h *Hotel, c image.Point, n int) (names []string) {
	for dy := range n {
		names = append(names, h.Map.TS.Tiles[h.Map.At(LayerDecor, c.X, c.Y+dy)].Name)
	}
	return names
}

// A Corridor Mirror hangs on the Room's front wall and survives the Room being landed again.
func TestCorridorMirror(t *testing.T) {
	h := buildGrid(t, 2, 3)
	h.SetCorridorMirrors([]CorridorMirror{
		{Floor: 1, Col: 1, West: true},
		{Floor: 2, Col: 0},
		{Floor: 0, Col: 0}, // the ground corridor has the Lobby Mirror
		{Floor: 3, Col: 0}, // no such floor
	})
	if len(h.Corridors) != 2 {
		t.Fatalf("%d Corridor Mirrors, want 2 (floors 1 and 2)", len(h.Corridors))
	}
	types, _ := LoadRoomTypes(assets.FS, h.Map.TS)
	for _, m := range h.Corridors {
		r := h.Rooms[m.Room]
		if !m.Glass.In(r.Shell()) || m.Spot.Min != image.Pt(m.Glass.Min.X, m.Glass.Max.Y) {
			t.Errorf("floor %d: glass %v, spot %v: want the glass in Room %s and the spot just below it", m.Floor, m.Glass, m.Spot, r.Number)
		}
		if m.Spot.Dx() != TileSize || m.Spot.Dy() != TileSize || h.Map.Solid(m.Cell.X, m.Cell.Y+2) {
			t.Errorf("floor %d: spot %v: want one walkable Corridor cell", m.Floor, m.Spot)
		}
		h.Land(m.Room, types[0], false)
		if n := decorNames(h, m.Cell, 2); !strings.HasPrefix(n[0], mirrorTile+"r0") || !strings.HasPrefix(n[1], mirrorTile+"r1") {
			t.Errorf("floor %d: after Land the wall holds %v, want the Mirror still hung", m.Floor, n)
		}
	}
	if a, b := h.Corridors[0], h.Corridors[1]; a.Floor != 1 || a.Room != 1*2+1 || b.Room != 0 || a.Cell.X >= h.Rooms[a.Room].Origin.X+RoomW/2 {
		t.Errorf("rooms %d and %d, west cell %v: floor 1 (row 1) column 1 west, floor 2 (the top row) column 0", a.Room, b.Room, a.Cell)
	}
}
