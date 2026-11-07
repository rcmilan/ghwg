package world

import (
	"errors"
	"fmt"
	"image"
	"math/rand/v2"
	"slices"
	"strconv"
)

// The hotel is generated from a grid of Rooms (D54). Cols x Rows is the shipped grid; Build takes any.
const (
	Cols, Rows   = 2, 1 // D138
	corridor     = 2    // Corridor width: between Room columns, between Room rows and above the Hall Wall (D55)
	lobbyRows    = 6    // floor rows between the Hall Wall and the front face (D56)
	behindRows   = 2    // Lobby rows behind the counter (D40)
	streetRows   = 3
	archwayWidth = 4 // on even map widths; 5 on odd ones, so it stays centered (D55)
	counterW     = 5 // counter cells: counter_left, counter_mid..., computer, counter_right (D87)
)

// layout is the hotel's geometry in map cells, derived from the grid (context/02-rooms/index.md D54-D57).
type layout struct {
	cols, rows int
	w, h       int
	hall       int // Hall Wall face (2 rows); the Lobby Archway is centered in it
	counter    int // counter row
	front      int // exterior south face (2 rows); the Entrance is centered in it
	street     int
	archway    int // the Archway's west column
	archwayW   int
	entrance   int // the Entrance's west column
	counterX   int // counter_left: just east of the Archway
}

func newLayout(cols, rows int) layout {
	l := layout{cols: cols, rows: rows, w: 2 + cols*RoomW + (cols-1)*corridor} // +2: the void margin columns
	l.hall = 1 + rows*(RoomH+corridor)
	l.counter = l.hall + 2 + behindRows
	l.front = l.hall + 2 + lobbyRows
	l.street = l.front + 2
	l.h = l.street + streetRows
	l.archwayW = archwayWidth + l.w%2
	l.archway, l.entrance = (l.w-l.archwayW)/2, l.w/2-1 // centered (D55); the Entrance sits one cell left of center on odd widths
	l.counterX = l.archway + l.archwayW
	return l
}

// origin is the top-left cell of the Room at grid column c, row r; Rooms sit flush against the outer walls.
func (l layout) origin(c, r int) image.Point {
	return image.Pt(1+c*(RoomW+corridor), 1+r*(RoomH+corridor))
}

// Room is one of the hotel's guest rooms, numbered floor = hundreds from the front (D57, amended in 03's
// S1 review): the row nearest the Entrance is 101, 102, …, the back row the highest. Built from a Room Type.
type Room struct {
	Number string
	Type   RoomType
	Origin image.Point       // cell of the room's top-left corner
	Door   image.Point       // cell of the door's top half; the bottom half is Door+(0,1)
	Stays  []image.Point     // Stay Spots, world px; len is the Room's capacity (D80)
	Lanes  []image.Rectangle // each Stay Spot's bed, world px (D167)
}

// Hotel is the built map plus the anchors the game needs. Points are world px.
type Hotel struct {
	l              layout
	Map            *Tilemap
	Rooms          []Room          // in map order, back row first: the Terminal draws them as they stand
	Doors          []Door          // one per Room (same index), then the Entrance
	Spawn          image.Point     // on the street below the Entrance, where guests appear
	Exit           image.Point     // where guests leave, a cell west of Spawn: the southbound lane, keeping right (D89)
	Archway        image.Point     // center of the Lobby Archway, on the Hall Wall's bottom row
	ConciergeStart image.Point     // behind the counter's computer
	Terminal       image.Rectangle // the computer and the cell on either side of it, both sides of the counter
	Mirror         image.Point     // where the Concierge stands to shift dimensions: below the Lobby mirror (D142)
	MirrorSpot     image.Rectangle // where SPACE shifts: the cells in front of the mirror and either side
	Lights         []image.Point   // ceiling lights over the Corridors (D95)
	Lobby          image.Rectangle // the Lobby, Hall Wall face included: lit as a whole at night (S6 review)
}

// cellCenter is the world-px center of cell (x,y).
func cellCenter(x, y int) image.Point { return image.Pt(x*TileSize+TileSize/2, y*TileSize+TileSize/2) }

// Build stamps the whole hotel for a cols x rows grid of Rooms, each Room's type drawn with Draw.
func Build(ts *Tileset, types []RoomType, cols, rows int, rng *rand.Rand) (*Hotel, error) {
	if len(types) == 0 {
		return nil, errors.New("world: no room types")
	}
	var missing []string
	id := func(name string) TileID {
		t, ok := ts.ID(name)
		if !ok {
			missing = append(missing, name)
		}
		return t
	}
	l := newLayout(cols, rows)
	m := NewTilemap(ts, l.w, l.h)
	put := func(l, x, y int, name string) { m.Set(l, x, y, id(name)) }
	// face: a 2-row wall face over cols x0..x1; gaps are openings (lintel on top, floor below).
	face := func(x0, x1, y int, gaps ...int) {
		in := func(x int) bool { return slices.Contains(gaps, x) }
		for x := x0; x <= x1; x++ {
			if in(x) {
				put(LayerWalls, x, y, "wall_lintel")
				continue
			}
			left, right := x == x0 || in(x-1), x == x1 || in(x+1)
			kind := "wall_mid"
			switch {
			case left && right:
				kind = "wall_single"
			case left:
				kind = "wall_left"
			case right:
				kind = "wall_right"
			}
			put(LayerWalls, x, y, kind+"_top")
			put(LayerWalls, x, y+1, kind+"_bottom")
		}
	}

	for y := range l.h {
		for x := range l.w {
			n := "floor_clay"
			switch {
			case y >= l.street:
				n = "floor_street" // full width, margin included
			case y == 0 || x == 0 || x == l.w-1:
				n = "void"
			}
			put(LayerFloor, x, y, n)
		}
	}
	for c := 1; c < cols; c++ { // the hotel's north wall atop each column Corridor (D55)
		x := l.origin(c, 0).X - corridor
		face(x, x+corridor-1, 1)
	}

	h := &Hotel{l: l, Map: m}
	for r := range rows {
		for c := range cols {
			o := l.origin(c, r)
			h.Rooms = append(h.Rooms, Room{
				Number: strconv.Itoa((rows-r)*100 + c + 1),
				Origin: o,
				Door:   o.Add(image.Pt(types[0].DoorCol, doorRow)), // every type shares it (D45)
			})
		}
	}

	archway, entrance := l.archway, l.entrance
	var gaps []int
	for x := archway; x < archway+l.archwayW; x++ {
		gaps = append(gaps, x)
	}
	face(1, l.w-2, l.hall, gaps...)               // Hall Wall + Lobby Archway
	face(1, l.w-2, l.front, entrance, entrance+1) // exterior face + Entrance
	for i, n := range []string{"entrance_left", "entrance_right"} {
		put(LayerDecor, entrance+i, l.front, n+"_top")
		put(LayerDecor, entrance+i, l.front+1, n+"_bottom")
	}
	// front windows (D67, S2 review): 2 cells wide, 2 per side, mirrored about the Entrance and evenly
	// spaced along the wall between the side line (x=1) and the Entrance; a spare cell widens the middle gap
	spare := entrance - 2 - 2*2 // free wall cells in x=2..entrance-1, around the 2 windows
	a := 2 + spare/3
	b := a + 2 + spare - 2*(spare/3)
	for _, d := range []int{entrance - a, entrance - b} {
		for _, x := range []int{entrance - d, entrance + d} {
			for c := range 4 {
				put(LayerDecor, x+c%2, l.front+c/2, fmt.Sprintf("prop_window_front_r%dc%d", c/2, c%2))
			}
		}
	}
	for col := 1; col < cols; col++ { // stairs (D69): in every column Corridor, at the foot of every Room row but the first (the top floor)
		x := l.origin(col, 0).X - corridor
		for r := 1; r < rows; r++ {
			for c := range 4 { // beside the Rooms' south faces, onto the Corridor below
				put(LayerDecor, x+c%2, l.origin(0, r).Y+doorRow+c/2, fmt.Sprintf("prop_stairs_r%dc%d", c/2, c%2))
			}
		}
	}
	computer := l.counterX + counterW - 2
	for x := l.counterX; x < l.counterX+counterW; x++ {
		n := "counter_mid"
		switch x {
		case l.counterX:
			n = "counter_left"
		case computer:
			n = "counter_computer_bottom"
		case computer + 1:
			n = "counter_right"
		}
		put(LayerDecor, x, l.counter, n)
	}
	put(LayerDecor, computer, l.counter-1, "counter_computer_top")
	mirror := archway - 2 // on the Hall Wall face, west of the Archway (D142)
	put(LayerDecor, mirror, l.hall, "prop_mirror_r0c0")
	put(LayerDecor, mirror, l.hall+1, "prop_mirror_r1c0")
	// hotel outer side walls: one unbroken line over every wall that meets them; Rooms never stamp over it
	for x, side := range map[int]string{1: "west", l.w - 2: "east"} {
		put(LayerDecor, x, 1, "border_"+side+"_top")
		for y := 2; y < l.front; y++ {
			put(LayerDecor, x, y, "border_"+side)
		}
		put(LayerDecor, x, l.front, "border_"+side+"_corner")
	}
	for i, r := range h.Rooms {
		for c := range 6 { // a doormat on the Corridor, flush below the door (D70)
			put(LayerDecor, r.Door.X-1+c%3, r.Door.Y+2+c/3, fmt.Sprintf("prop_doormat_r%dc%d", c/3, c%3))
		}
		top := r.Door.Mul(TileSize)
		h.Doors = append(h.Doors, Door{Room: i, Rect: image.Rectangle{top, top.Add(image.Pt(TileSize, 2*TileSize))}})
	}
	h.Doors = append(h.Doors, Door{Room: -1, Rect: image.Rect(entrance*TileSize, l.front*TileSize, (entrance+2)*TileSize, (l.front+2)*TileSize)})
	if len(missing) > 0 {
		return nil, fmt.Errorf("world: tileset lacks tiles %v", missing)
	}
	h.Furnish(Draw(types, len(h.Rooms), rng))

	h.Spawn = image.Pt((entrance+1)*TileSize, (l.street+1)*TileSize+TileSize/2) // on the Entrance's center line
	h.Exit = h.Spawn.Sub(image.Pt(TileSize, 0))
	h.Archway = image.Pt(archway*TileSize+l.archwayW*TileSize/2, (l.hall+1)*TileSize)
	h.ConciergeStart = cellCenter(computer, l.counter-2)
	// ceiling lights (D95), on the grid: one in the Corridor below each Room and in the column Corridor beside it
	for c := range cols {
		for r := range rows {
			o := l.origin(c, r).Mul(TileSize)
			h.Lights = append(h.Lights, o.Add(image.Pt(RoomW*TileSize/2, (RoomH+corridor/2)*TileSize)))
			if c > 0 {
				h.Lights = append(h.Lights, o.Add(image.Pt(-corridor*TileSize/2, RoomH*TileSize/2)))
			}
		}
	}
	h.Lobby = image.Rect(TileSize, l.hall*TileSize, (l.w-1)*TileSize, l.front*TileSize)
	h.Terminal = image.Rect((computer-1)*TileSize, (l.counter-2)*TileSize, (computer+2)*TileSize, (l.counter+2)*TileSize)
	h.Mirror = cellCenter(mirror, l.hall+2)
	h.MirrorSpot = image.Rect((mirror-1)*TileSize, (l.hall+2)*TileSize, (mirror+2)*TileSize, (l.hall+3)*TileSize)
	return h, nil
}

// Draw picks each Room's Room Type for a Run, independently, repeats allowed (D46): a tier, then a bed layout
// of that tier, then one of its variants, equal odds at each step (D84).
func Draw(types []RoomType, n int, rng *rand.Rand) []RoomType {
	pick := func(ts []RoomType, key func(RoomType) string) []RoomType { // the types sharing a random key
		var keys []string
		for _, t := range ts {
			if !slices.Contains(keys, key(t)) {
				keys = append(keys, key(t))
			}
		}
		k := keys[rng.IntN(len(keys))]
		return slices.DeleteFunc(slices.Clone(ts), func(t RoomType) bool { return key(t) != k })
	}
	next := make([]RoomType, n)
	for i := range next {
		vs := pick(pick(types, func(t RoomType) string { return t.Name }), func(t RoomType) string { return t.Beds })
		next[i] = vs[rng.IntN(len(vs))]
	}
	return next
}

// Furnish gives every Room its Room Type, one per Room, and stamps its cells, tiles and collisions under
// the hotel's outer side lines. Doors never move (D45), so the Doors and every anchor stay valid.
func (h *Hotel) Furnish(types []RoomType) {
	for i := range h.Rooms {
		r := &h.Rooms[i]
		r.Type, r.Stays, r.Lanes = types[i], nil, nil
		for _, p := range types[i].Stays {
			r.Stays = append(r.Stays, r.Origin.Mul(TileSize).Add(p))
		}
		for _, b := range types[i].Lanes {
			r.Lanes = append(r.Lanes, b.Add(r.Origin.Mul(TileSize)))
		}
		for layer, cells := range r.Type.Tiles {
			for c, tid := range cells {
				if x := r.Origin.X + c%RoomW; layer != LayerDecor || !h.OuterLine(x) {
					h.Map.Set(layer, x, r.Origin.Y+c/RoomW, tid)
				}
			}
		}
	}
}

// OuterLine: column x carries a hotel outer side line in its decor layer.
func (h *Hotel) OuterLine(x int) bool { return x == 1 || x == h.l.w-2 }

// Grid is the hotel's Room grid, cols × rows.
func (h *Hotel) Grid() (cols, rows int) { return h.l.cols, h.l.rows }
