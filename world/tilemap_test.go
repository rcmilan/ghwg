package world

import (
	"image"
	"testing"
)

func TestBlockedBox(t *testing.T) {
	ts := loadTileset(t)
	m := NewTilemap(ts, 3, 1)
	set := func(x int, name string) {
		id, ok := ts.ID(name)
		if !ok {
			t.Fatalf("no tile %q", name)
		}
		m.Set(LayerDecor, x, 0, id)
	}
	set(0, "counter_mid") // rect (0,8,16,8): front strip
	set(1, "border_west") // rect (0,0,6,16)
	set(2, "border_east") // rect (10,0,6,16)
	// ~10x6 foot box
	foot := func(x, y int) image.Rectangle { return image.Rect(x, y, x+10, y+6) }

	cases := []struct {
		name string
		box  image.Rectangle
		want bool
	}{
		{"behind the counter (top half)", foot(3, 2), false},
		{"on the counter front strip", foot(3, 5), true},
		{"just right of the west side line", foot(16+6, 4), false},
		{"1px into the west side line", foot(16+5, 4), true},
		{"just left of the east side line", foot(32, 4), false},
		{"1px into the east side line", foot(32+1, 4), true},
		{"outside the map", foot(-1, 2), true},
	}
	for _, c := range cases {
		if got := m.BlockedBox(c.box); got != c.want {
			t.Errorf("%s %v: blocked=%v, want %v", c.name, c.box, got, c.want)
		}
	}
}

// A cell sorts at its object's footprint: a 2-row face, the lintel over an opening and the computer
// all sort at their bottom row's lower edge, so an actor's feet above that line put it behind them.
func TestSortY(t *testing.T) {
	h := buildHotel(t)
	m := h.Map
	l := newLayout(2, 2)
	hallRow, counterRow := l.hall, l.counter
	seg, comp := l.counterX+1, l.counterX+l.counterW-2 // a counter_mid cell; the computer
	foot := func(row int) int { return (row + 1) * TileSize }
	cases := []struct {
		name string
		x, y int
		want int
	}{
		{"hall wall face, top row", 5, hallRow, foot(hallRow + 1)},
		{"hall wall face, bottom row", 5, hallRow + 1, foot(hallRow + 1)},
		{"Lobby Archway lintel", 15, hallRow, foot(hallRow + 1)},
		{"counter segment", seg, counterRow, foot(counterRow)},
		{"computer, top cell", comp, counterRow - 1, foot(counterRow)},
		{"computer, bottom cell", comp, counterRow, foot(counterRow)},
	}
	for _, c := range cases {
		if got := m.SortY(c.x, c.y); got != c.want {
			t.Errorf("%s (%d,%d): sorts at y %d, want %d", c.name, c.x, c.y, got, c.want)
		}
	}
	// the deepest foot box behind the counter (on its walkable top half) sorts before it; the shallowest
	// one in front (just below its solid front strip) sorts after it
	key := m.SortY(seg, counterRow)
	behind := image.Rect(seg*TileSize, counterRow*TileSize+2, seg*TileSize+10, counterRow*TileSize+8)
	front := behind.Add(image.Pt(0, TileSize-2))
	if m.BlockedBox(behind) || m.BlockedBox(front) {
		t.Fatalf("test foot boxes %v / %v should be walkable", behind, front)
	}
	if mid := (behind.Min.Y + behind.Max.Y) / 2; mid >= key {
		t.Errorf("behind the counter, feet y %d should sort before the counter (%d)", mid, key)
	}
	if mid := (front.Min.Y + front.Max.Y) / 2; mid < key {
		t.Errorf("in front of the counter, feet y %d should sort after the counter (%d)", mid, key)
	}
}

// A side line is one long wall: each of its cells sorts at the line's south end (the face its corner
// sits on), so an actor beside it is behind the whole line or in front of all of it, never split by row.
// The face under the line keeps its own sort y.
func TestTileSortYSideLines(t *testing.T) {
	m := buildHotel(t).Map
	l := newLayout(2, 2)
	hallRow, frontRow := l.hall, l.front
	cases := []struct {
		name   string
		x      int
		rows   []int
		bottom int // the run's last row (its corner, on a face's top row)
	}{
		{"hotel west wall", 1, []int{2, 10, 21, hallRow + 1, hallRow + 2, frontRow - 1, frontRow}, frontRow},
		{"room 101's east side", 9, []int{1, 3, 7, 8}, 8},
		{"room 201's east side", 9, []int{12, 16, 19}, 19},
	}
	for _, c := range cases {
		want := m.SortY(c.x, c.bottom)
		for _, y := range c.rows {
			if got := m.TileSortY(LayerDecor, c.x, y); got != want {
				t.Errorf("%s, row %d: side line sorts at %d, want %d (its south end)", c.name, y, got, want)
			}
		}
	}
	if got, want := m.TileSortY(LayerWalls, 1, hallRow), m.SortY(1, hallRow); got != want {
		t.Errorf("hall wall face under the west line sorts at %d, want its own %d", got, want)
	}
}

// D65: a rug (flat tile) sorts under every actor, even one standing just above its cell, whose sprite
// reaches below its feet into the rug (S7 review: a carpet covered a guest's feet).
func TestFlatSortsUnderActors(t *testing.T) {
	ts := loadTileset(t)
	id, ok := ts.ID("prop_carpet_a_r0c0")
	if !ok || !ts.Tiles[id].Flat {
		t.Fatal("prop_carpet_a_r0c0 missing or not flat")
	}
	m := NewTilemap(ts, 3, 3)
	m.Set(LayerProps, 1, 1, id)
	if got := m.TileSortY(LayerProps, 1, 1); got > 0 {
		t.Errorf("rug sorts at %d, want 0: under any actor's feet", got)
	}
}

// D65: a prop's base belongs to the prop. A bed against the back wall (its top row on a window cell, base
// 2) leaves the curtain sorting with its wall (S7 review: the Concierge got behind a Deluxe curtain).
func TestPropBaseStaysOwn(t *testing.T) {
	ts := loadTileset(t)
	id := func(n string) TileID {
		v, ok := ts.ID(n)
		if !ok {
			t.Fatalf("no tile %s", n)
		}
		return v
	}
	m := NewTilemap(ts, 3, 5)
	m.Set(LayerWalls, 1, 1, id("wall_wood_mid_bottom"))
	m.Set(LayerDecor, 1, 1, id("window_curtain_r1c0"))
	curtain := m.TileSortY(LayerDecor, 1, 1)
	bed := id("prop_bed_brown_r0c0")
	m.Set(LayerProps, 1, 1, bed)
	if got := m.TileSortY(LayerDecor, 1, 1); got != curtain {
		t.Errorf("curtain sorts at %d under a bed's top row, want %d as without it", got, curtain)
	}
	if got, want := m.TileSortY(LayerProps, 1, 1), (1+ts.Tiles[bed].Base+1)*TileSize; got != want {
		t.Errorf("bed's top row sorts at %d, want %d (its own base)", got, want)
	}
}
