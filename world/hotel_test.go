package world

import (
	"fmt"
	"image"
	"math/rand/v2"
	"slices"
	"strings"
	"testing"

	"ghosthotel/assets"
)

func buildHotel(t *testing.T) *Hotel { return buildGrid(t, Cols, Rows) }

func buildGrid(t *testing.T, cols, rows int) *Hotel {
	t.Helper()
	ts := loadTileset(t)
	types, err := LoadRoomTypes(assets.FS, ts)
	if err != nil {
		t.Fatal(err)
	}
	h, err := Build(ts, types, cols, rows, rand.New(rand.NewPCG(1, 2)))
	if err != nil {
		t.Fatal(err)
	}
	return h
}

// forGrids runs f on every Room grid the hotel must support (D54): 2×2 up to 3×3.
func forGrids(t *testing.T, f func(t *testing.T, h *Hotel, l layout)) {
	for cols := 2; cols <= 3; cols++ {
		for rows := 2; rows <= 3; rows++ {
			t.Run(fmt.Sprintf("%dx%d", cols, rows), func(t *testing.T) { f(t, buildGrid(t, cols, rows), newLayout(cols, rows)) })
		}
	}
}

func TestHotelLayout(t *testing.T) {
	h := buildHotel(t)
	if h.Map.W != 30 || h.Map.H != 36 {
		t.Fatalf("map %dx%d, want 30x36", h.Map.W, h.Map.H)
	}
	want := map[string]image.Point{"101": {1, 1}, "102": {16, 1}, "201": {1, 12}, "202": {16, 12}}
	if len(h.Rooms) != len(want) {
		t.Fatalf("%d rooms, want 4", len(h.Rooms))
	}
	for _, r := range h.Rooms {
		if r.Origin != want[r.Number] {
			t.Errorf("room %s origin %v, want %v", r.Number, r.Origin, want[r.Number])
		}
	}
	forGrids(t, func(t *testing.T, h *Hotel, l layout) {
		for name, p := range map[string]image.Point{"spawn": h.Spawn, "concierge": h.ConciergeStart, "pay spot": h.PaySpot, "pay exit": h.PayExit, "archway": h.Archway} {
			if h.Map.Solid(p.X/TileSize, p.Y/TileSize) {
				t.Errorf("%s %v is on a solid cell", name, p)
			}
		}
		// D57: row = hundreds, row-major; D56: 6 floor rows between the Hall Wall and the front face
		if len(h.Rooms) != l.cols*l.rows {
			t.Fatalf("%d rooms, want %d", len(h.Rooms), l.cols*l.rows)
		}
		if last, want := h.Rooms[len(h.Rooms)-1].Number, fmt.Sprintf("%d%02d", l.rows, l.cols); last != want {
			t.Errorf("last room is %s, want %s", last, want)
		}
		if got := l.front - (l.hall + 2); got != 6 {
			t.Errorf("lobby has %d floor rows, want 6", got)
		}
		// every door opens onto a walkable corridor cell (D55)
		for _, r := range h.Rooms {
			if below := r.Door.Add(image.Pt(0, 2)); h.Map.Solid(below.X, below.Y) {
				t.Errorf("room %s: door %v opens onto a solid cell", r.Number, r.Door)
			}
		}
	})
}

func TestHotelPaths(t *testing.T) {
	forGrids(t, func(t *testing.T, h *Hotel, _ layout) {
		for i, r := range h.Rooms {
			path := h.Path(h.Spawn, r.Stay, i)
			if path == nil {
				t.Fatalf("room %s: no path from spawn to its Stay Spot", r.Number)
			}
			if path[len(path)-1] != r.Stay {
				t.Errorf("room %s: path ends at %v, want %v", r.Number, path[len(path)-1], r.Stay)
			}
			cells := make([]image.Point, len(path))
			for k, p := range path {
				cells[k] = p.Div(TileSize)
			}
			if !slices.Contains(cells, r.Door) || !slices.Contains(cells, r.Door.Add(image.Pt(0, 1))) {
				t.Errorf("room %s: path does not pass its own door %v", r.Number, r.Door)
			}
			for j, other := range h.Rooms {
				if j != i && (slices.Contains(cells, other.Door) || slices.Contains(cells, other.Door.Add(image.Pt(0, 1)))) {
					t.Errorf("room %s: path crosses room %s's door", r.Number, other.Number)
				}
			}
			// the way back: Stay Spot -> pay spot -> street
			if h.Path(r.Stay, h.PaySpot, i) == nil || h.Path(h.PaySpot, h.PayExit, -1) == nil || h.Path(h.PayExit, h.Spawn, -1) == nil {
				t.Errorf("room %s: no way back to the pay spot and the street", r.Number)
			}
			// doors are the only way in: with no door passable, the Stay Spot is unreachable
			if h.Path(h.Spawn, r.Stay, -1) != nil {
				t.Errorf("room %s: Stay Spot reachable without its door", r.Number)
			}
		}
	})
}

// TestHotelASCII prints the built map; compare with index.md "Hotel layout" and preview_hotel.png.
// go test ./world -run TestHotelASCII -v
func TestHotelASCII(t *testing.T) {
	h := buildHotel(t)
	marks := map[image.Point]byte{h.Spawn.Div(TileSize): '@', h.PaySpot.Div(TileSize): 'P', h.ConciergeStart.Div(TileSize): 'B'}
	for _, r := range h.Rooms {
		marks[r.Stay.Div(TileSize)] = 'S'
	}
	var b strings.Builder
	b.WriteString("\n    ")
	for x := range h.Map.W {
		b.WriteByte("0123456789"[x%10])
	}
	b.WriteByte('\n')
	for y := range h.Map.H {
		fmt.Fprintf(&b, "%2d  ", y)
		for x := range h.Map.W {
			if c, ok := marks[image.Pt(x, y)]; ok {
				b.WriteByte(c)
				continue
			}
			b.WriteByte(asciiCell(h.Map, x, y))
		}
		b.WriteByte('\n')
	}
	b.WriteString("legend: ' ' void  . floor  : street  # wall  = lintel  | side line  w window  D door  E entrance  C counter  T computer  S stay  @ spawn  P pay  B concierge")
	t.Log(b.String())
}

func asciiCell(m *Tilemap, x, y int) byte {
	for _, l := range []int{LayerDecor, LayerWalls, LayerFloor} {
		n := m.TS.Tiles[m.At(l, x, y)].Name
		for _, p := range []struct {
			prefix string
			c      byte
		}{{"border_", '|'}, {"window_", 'w'}, {"door_", 'D'}, {"entrance_", 'E'}, {"counter_computer", 'T'}, {"counter_", 'C'},
			{"wall_lintel", '='}, {"wall_", '#'}, {"void", ' '}, {"floor_street", ':'}, {"floor_", '.'}} {
			if strings.HasPrefix(n, p.prefix) {
				return p.c
			}
		}
	}
	return '?'
}

// D29: the Terminal is reachable from both sides of the counter, and guests pay in front of it.
// D40: walkable ends beside the counter and 2 free rows behind it.
func TestHotelCounterSides(t *testing.T) {
	forGrids(t, func(t *testing.T, h *Hotel, l layout) {
		counterTop, counterFront := l.counter*TileSize, (l.counter+1)*TileSize
		if h.ConciergeStart.Y >= counterTop || h.PaySpot.Y < counterFront {
			t.Errorf("concierge start %v should be behind the counter (y < %d), pay spot %v in front (y >= %d)",
				h.ConciergeStart, counterTop, h.PaySpot, counterFront)
		}
		for name, p := range map[string]image.Point{"behind": h.ConciergeStart, "in front": h.PaySpot} {
			if !p.In(h.Terminal) {
				t.Errorf("terminal zone %v misses the %s spot %v", h.Terminal, name, p)
			}
		}
		for _, c := range []image.Point{{2, l.counter}, {8, l.counter}} {
			if h.Map.Solid(c.X, c.Y) {
				t.Errorf("counter end %v is solid", c)
			}
		}
		for y := l.counter - 2; y < l.counter; y++ {
			for x := 2; x <= 8; x++ {
				if h.Map.Solid(x, y) {
					t.Errorf("cell (%d,%d) behind the counter is solid", x, y)
				}
			}
		}
	})
}
