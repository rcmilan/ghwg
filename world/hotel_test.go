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

// buildHotel builds a 2×2 grid: the hand-measured coordinates in the pinned tests were taken on it,
// whatever grid ships (Cols, Rows).
func buildHotel(t *testing.T) *Hotel { return buildGrid(t, 2, 2) }

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

// forGrids runs f on every Room grid the hotel must support (D54): 2×1 up to 3×3.
func forGrids(t *testing.T, f func(t *testing.T, h *Hotel, l layout)) {
	for cols := 2; cols <= 3; cols++ {
		for rows := 1; rows <= 3; rows++ {
			t.Run(fmt.Sprintf("%dx%d", cols, rows), func(t *testing.T) { f(t, buildGrid(t, cols, rows), newLayout(cols, rows)) })
		}
	}
}

func TestHotelLayout(t *testing.T) {
	h := buildHotel(t)
	if h.Map.W != 22 || h.Map.H != 36 {
		t.Fatalf("map %dx%d, want 22x36", h.Map.W, h.Map.H)
	}
	want := map[string]image.Point{"101": {1, 1}, "102": {12, 1}, "201": {1, 12}, "202": {12, 12}}
	if len(h.Rooms) != len(want) {
		t.Fatalf("%d rooms, want 4", len(h.Rooms))
	}
	for _, r := range h.Rooms {
		if r.Origin != want[r.Number] {
			t.Errorf("room %s origin %v, want %v", r.Number, r.Origin, want[r.Number])
		}
	}
	forGrids(t, func(t *testing.T, h *Hotel, l layout) {
		for name, p := range map[string]image.Point{"spawn": h.Spawn, "concierge": h.ConciergeStart, "archway": h.Archway} {
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
	h := buildGrid(t, Cols, Rows) // the shipped grid
	marks := map[image.Point]byte{h.Spawn.Div(TileSize): '@', h.ConciergeStart.Div(TileSize): 'B'}
	for _, r := range h.Rooms {
		marks[r.Pay.Div(TileSize)] = 'P'
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

// D29: the Terminal is reachable from behind the counter. D62: the counter sits just east of the Archway
// and every Room has its own Pay Spot in front of it. D40: walkable ends and 2 free rows behind it.
func TestHotelCounterSides(t *testing.T) {
	forGrids(t, func(t *testing.T, h *Hotel, l layout) {
		if l.counterX != l.archway+archwayWidth || l.counterX+l.counterW > l.w-3 {
			t.Errorf("counter %d..%d, want it just east of the Archway (ends %d) and inside the hotel", l.counterX, l.counterX+l.counterW-1, l.archway+archwayWidth-1)
		}
		if h.ConciergeStart.Y >= l.counter*TileSize || !h.ConciergeStart.In(h.Terminal) {
			t.Errorf("concierge start %v should be behind the counter, in the terminal zone %v", h.ConciergeStart, h.Terminal)
		}
		seen := map[image.Point]bool{}
		for _, r := range h.Rooms {
			c := r.Pay.Div(TileSize)
			if c.Y != l.counter+1 || c.X < l.counterX || c.X >= l.counterX+l.counterW || h.Map.Solid(c.X, c.Y) || seen[c] {
				t.Errorf("room %s pays at %v, want its own walkable cell right in front of the counter", r.Number, c)
			}
			seen[c] = true
		}
		for _, x := range []int{l.counterX - 1, l.counterX + l.counterW} {
			if h.Map.Solid(x, l.counter) {
				t.Errorf("counter end (%d,%d) is solid", x, l.counter)
			}
		}
		for y := l.counter - 2; y < l.counter; y++ {
			for x := l.counterX - 1; x <= l.counterX+l.counterW; x++ {
				if h.Map.Solid(x, y) {
					t.Errorf("cell (%d,%d) behind the counter is solid", x, y)
				}
			}
		}
	})
}

// D62: each Room's own lane reaches its Pay Spot and the Street, and a guest's way to pay never crosses
// the Pay Spot or the way out of a guest who left before it (lower Room index).
func TestCheckoutLanes(t *testing.T) {
	forGrids(t, func(t *testing.T, h *Hotel, l layout) {
		cells := func(path []image.Point) map[image.Point]bool {
			m := map[image.Point]bool{}
			for _, p := range path {
				m[p.Div(TileSize)] = true
			}
			return m
		}
		pays, outs := make([][]image.Point, len(h.Rooms)), make([][]image.Point, len(h.Rooms))
		for i, r := range h.Rooms {
			pays[i], outs[i] = h.Checkout(i)
			if len(pays[i]) == 0 || pays[i][len(pays[i])-1] != r.Pay || len(outs[i]) == 0 || outs[i][len(outs[i])-1] != h.Spawn {
				t.Fatalf("room %s: lane does not reach its Pay Spot %v and the Street %v", r.Number, r.Pay, h.Spawn)
			}
		}
		for j := range h.Rooms {
			later := cells(pays[j])
			for i := range j {
				for c := range cells(append([]image.Point{h.Rooms[i].Pay}, outs[i]...)) {
					if later[c] {
						t.Errorf("room %s's way to pay crosses room %s's Pay Spot or way out at %v", h.Rooms[j].Number, h.Rooms[i].Number, c)
					}
				}
			}
		}
	})
}
