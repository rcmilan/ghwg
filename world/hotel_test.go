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
	want := map[string]image.Point{"201": {1, 1}, "202": {12, 1}, "101": {1, 12}, "102": {12, 12}}
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
		// D57: floor = hundreds from the front, back row first; D56: 6 floor rows between the Hall Wall and the front face
		if len(h.Rooms) != l.cols*l.rows {
			t.Fatalf("%d rooms, want %d", len(h.Rooms), l.cols*l.rows)
		}
		if first, last := h.Rooms[0].Number, h.Rooms[len(h.Rooms)-1].Number; first != fmt.Sprintf("%d01", l.rows) || last != fmt.Sprintf("1%02d", l.cols) {
			t.Errorf("rooms run %s..%s, want %d01 at the back left to 1%02d at the front right", first, last, l.rows, l.cols)
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
			stay := r.Stays[0]
			path := h.Path(h.Spawn, stay, i)
			if path == nil {
				t.Fatalf("room %s: no path from spawn to its Stay Spot", r.Number)
			}
			if path[len(path)-1] != stay {
				t.Errorf("room %s: path ends at %v, want %v", r.Number, path[len(path)-1], stay)
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
			if h.Path(h.Spawn, stay, -1) != nil {
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
	for _, q := range h.Queue {
		marks[q.Div(TileSize)] = 'Q'
	}
	for _, r := range h.Rooms {
		for _, s := range r.Stays {
			marks[s.Div(TileSize)] = 'S'
		}
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
	b.WriteString("legend: ' ' void  . floor  : street  # wall  = lintel  | side line  w window  D door  E entrance  C counter  T computer  S stay  @ spawn  Q queue  B concierge")
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

// D29: the Terminal is reachable from behind the counter, which sits just east of the Archway. D78: the
// Queue's lanes start on walkable, distinct cells right in front of the counter. D40: walkable ends and 2
// free rows behind it.
func TestHotelCounterSides(t *testing.T) {
	forGrids(t, func(t *testing.T, h *Hotel, l layout) {
		if l.counterX != l.archway+l.archwayW || l.counterX+counterW > l.w-3 {
			t.Errorf("counter %d..%d, want it just east of the Archway (ends %d) and inside the hotel", l.counterX, l.counterX+counterW-1, l.archway+l.archwayW-1)
		}
		if h.ConciergeStart.Y >= l.counter*TileSize || !h.ConciergeStart.In(h.Terminal) {
			t.Errorf("concierge start %v should be behind the counter, in the terminal zone %v", h.ConciergeStart, h.Terminal)
		}
		seen := map[image.Point]bool{}
		for _, q := range h.Queue {
			c := q.Div(TileSize)
			if c.Y != l.counter+1 || c.X < l.counterX || c.X >= l.counterX+counterW || h.Map.Solid(c.X, c.Y) || seen[c] {
				t.Errorf("queue lane at %v: want a distinct walkable cell right in front of the counter", c)
			}
			seen[c] = true
		}
		if len(h.Queue) != queueLanes {
			t.Errorf("%d queue lanes, want %d", len(h.Queue), queueLanes)
		}
		// D55: the Archway is centered in the Hall Wall, between the side lines at 1 and w-2
		if left, right := l.archway-2, l.w-3-(l.archway+l.archwayW-1); left != right {
			t.Errorf("archway %d..%d: %d wall cells to its left, %d to its right", l.archway, l.archway+l.archwayW-1, left, right)
		}
		for _, x := range []int{l.counterX - 1, l.counterX + counterW} {
			if h.Map.Solid(x, l.counter) {
				t.Errorf("counter end (%d,%d) is solid", x, l.counter)
			}
		}
		for y := l.counter - 2; y < l.counter; y++ {
			for x := l.counterX - 1; x <= l.counterX+counterW; x++ {
				if h.Map.Solid(x, y) {
					t.Errorf("cell (%d,%d) behind the counter is solid", x, y)
				}
			}
		}
	})
}

// D66: a wall face's top row is walkable, its bottom row solid. An actor walking up to a wall from behind
// (north) steps into the top row until its feet meet the bottom row: the face hides its lower 16 px, half
// of a 32 px sprite. From the front the face still blocks at its bottom row.
func TestBehindWall(t *testing.T) {
	h := buildHotel(t)
	o := h.Rooms[0].Origin
	x, top := o.X+2, o.Y+doorRow                                                                   // the Room's south face, west of the door
	foot := func(y int) image.Rectangle { return image.Rect(x*TileSize+3, y-6, x*TileSize+13, y) } // feet box, bottom at y
	deepest := (top + 1) * TileSize
	if h.Map.BlockedBox(foot(deepest)) {
		t.Fatalf("feet in the top row of a wall face at (%d,%d) are blocked", x, top)
	}
	if !h.Map.BlockedBox(foot(deepest + 1)) {
		t.Errorf("feet reach into the face's bottom row at (%d,%d)", x, top+1)
	}
	if sy := h.Map.TileSortY(LayerWalls, x, top); deepest-3 >= sy { // feet point: the box's center
		t.Errorf("an actor with feet at %d sorts in front of the face (sort y %d)", deepest-3, sy)
	}
	if h.Path(h.Rooms[0].Stays[0], cellCenter(x, top), 0) == nil {
		t.Errorf("the top row of the south face at (%d,%d) is not reachable from the Stay Spot", x, top)
	}
}

// D67: windows on the front face, mirrored about the Entrance, clear of it and of the side lines.
func TestFrontWindows(t *testing.T) {
	forGrids(t, func(t *testing.T, h *Hotel, l layout) {
		win := func(x int) bool {
			return strings.HasPrefix(h.Map.TS.Tiles[h.Map.At(LayerDecor, x, l.front)].Name, "prop_window_front")
		}
		n := 0
		for x := range l.w {
			if !win(x) {
				continue
			}
			n++
			if x < 2 || x > l.w-3 || x == l.entrance-1 || x == l.entrance+2 || !win(2*l.entrance+1-x) {
				t.Errorf("window cell at x=%d: touches a side line or the Entrance, or has no mirror", x)
			}
		}
		if n == 0 {
			t.Error("no windows on the front face")
		}
	})
}

// D66: a side line on a face's top row is walkable too: from behind a Room's back face an actor walks on
// into the column Corridor. On the south face's top row it still blocks, or it would be a way out of the Room.
func TestBehindSideLine(t *testing.T) {
	h := buildHotel(t)
	o := h.Rooms[2].Origin // 101: below the back Room row, its back face open to the Corridor above
	x := o.X + RoomW - 1   // its east side line
	foot := func(y int) image.Rectangle { return image.Rect((x+1)*TileSize-5, y-6, (x+1)*TileSize+5, y) }
	if h.Map.BlockedBox(foot((o.Y + 1) * TileSize)) {
		t.Errorf("behind the back face, the side line at (%d,%d) blocks", x, o.Y)
	}
	if !h.Map.BlockedBox(foot((o.Y + doorRow + 1) * TileSize)) {
		t.Errorf("behind the south face, the side line at (%d,%d) lets actors out of the Room", x, o.Y+doorRow)
	}
}

// D69: stairs (2x2) fill every column Corridor at the foot of every Room row but the first, and stay walkable.
func TestStairs(t *testing.T) {
	forGrids(t, func(t *testing.T, h *Hotel, l layout) {
		var at []image.Point
		for y := range l.h {
			for x := range l.w {
				if strings.HasPrefix(h.Map.TS.Tiles[h.Map.At(LayerDecor, x, y)].Name, "prop_stairs") {
					at = append(at, image.Pt(x, y))
					if h.Map.Solid(x, y) {
						t.Errorf("stair cell (%d,%d) is solid", x, y)
					}
				}
			}
		}
		if len(at) != 4*(l.cols-1)*(l.rows-1) {
			t.Fatalf("%d stair cells, want 4 per column Corridor per Room row below the first", len(at))
		}
		for col := 1; col < l.cols; col++ {
			for r := 1; r < l.rows; r++ {
				if p := image.Pt(l.origin(col, 0).X-corridor, l.origin(0, r).Y+doorRow); !slices.Contains(at, p) {
					t.Errorf("no stairs at %v, Corridor %d at the foot of Room row %d", p, col, r)
				}
			}
		}
	})
}

// D70: a doormat on the Corridor below every Room's door, walkable.
func TestDoormats(t *testing.T) {
	forGrids(t, func(t *testing.T, h *Hotel, l layout) {
		for _, r := range h.Rooms {
			p := r.Door.Add(image.Pt(0, 2))
			if n := h.Map.TS.Tiles[h.Map.At(LayerDecor, p.X, p.Y)].Name; !strings.HasPrefix(n, "prop_doormat") || h.Map.Solid(p.X, p.Y) {
				t.Errorf("room %s: %q at %v below its door, want a walkable doormat", r.Number, n, p)
			}
		}
	})
}
