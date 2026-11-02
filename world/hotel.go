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
	Cols, Rows   = 3, 3
	corridor     = 2 // Corridor width: between Room columns, between Room rows and above the Hall Wall (D55)
	lobbyRows    = 6 // floor rows between the Hall Wall and the front face (D56)
	behindRows   = 2 // Lobby rows behind the counter (D40)
	streetRows   = 3
	archwayWidth = 4
	minCounter   = 5 // counter cells: counter_left, counter_mid..., computer, counter_right
)

// layout is the hotel's geometry in map cells, derived from the grid (context/02-rooms/index.md D54-D57).
type layout struct {
	cols, rows int
	w, h       int
	hall       int // Hall Wall face (2 rows); the Lobby Archway is centered in it
	counter    int // counter row; below it: the pay row, the approach row, the exit row (D62)
	front      int // exterior south face (2 rows); the Entrance is centered in it
	street     int
	archway    int // the Archway's west column
	entrance   int // the Entrance's west column
	counterX   int // counter_left: just east of the Archway
	counterW   int // one cell per Room at least, so each Room has its own Pay Spot (D62)
}

func newLayout(cols, rows int) layout {
	l := layout{cols: cols, rows: rows, w: 2 + cols*RoomW + (cols-1)*corridor} // +2: the void margin columns
	l.hall = 1 + rows*(RoomH+corridor)
	l.counter = l.hall + 2 + behindRows
	l.front = l.hall + 2 + lobbyRows
	l.street = l.front + 2
	l.h = l.street + streetRows
	l.archway, l.entrance = l.w/2-archwayWidth/2, l.w/2-1 // centered (D55); one cell left of center on odd widths
	l.counterX, l.counterW = l.archway+archwayWidth, max(minCounter, cols*rows)
	return l
}

// origin is the top-left cell of the Room at grid column c, row r; Rooms sit flush against the outer walls.
func (l layout) origin(c, r int) image.Point {
	return image.Pt(1+c*(RoomW+corridor), 1+r*(RoomH+corridor))
}

// Room is one of the hotel's guest rooms, numbered row = hundreds (D57), built from a Room Type.
type Room struct {
	Number string
	Type   RoomType
	Origin image.Point // cell of the room's top-left corner
	Door   image.Point // cell of the door's top half; the bottom half is Door+(0,1)
	Stay   image.Point // Stay Spot, world px
	Pay    image.Point // this Room's own Pay Spot in front of the counter, world px (D62)
}

// Hotel is the built map plus the anchors the game needs. Points are world px.
type Hotel struct {
	l              layout
	Map            *Tilemap
	Rooms          []Room
	Doors          []Door          // one per Room (same index), then the Entrance
	Spawn          image.Point     // on the street below the Entrance; guests also leave here
	Archway        image.Point     // center of the Lobby Archway, on the Hall Wall's bottom row
	ConciergeStart image.Point     // behind the counter's computer
	LaneEntry      image.Point     // where every Room's lane joins: the Archway's east column, on the approach row (D62)
	Terminal       image.Rectangle // the computer and the cell on either side of it, both sides of the counter
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
				Number: strconv.Itoa((r+1)*100 + c + 1),
				Origin: o,
				Door:   o.Add(image.Pt(types[0].DoorCol, doorRow)), // every type shares it (D45)
			})
		}
	}

	archway, entrance := l.archway, l.entrance
	var gaps []int
	for x := archway; x < archway+archwayWidth; x++ {
		gaps = append(gaps, x)
	}
	face(1, l.w-2, l.hall, gaps...)               // Hall Wall + Lobby Archway
	face(1, l.w-2, l.front, entrance, entrance+1) // exterior face + Entrance
	for i, n := range []string{"entrance_left", "entrance_right"} {
		put(LayerDecor, entrance+i, l.front, n+"_top")
		put(LayerDecor, entrance+i, l.front+1, n+"_bottom")
	}
	for d := 3; entrance-d >= 3; d += 3 { // front windows (D67): 2 cells wide, every 3 cells out from the Entrance
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
	computer := l.counterX + l.counterW - 2
	for x := l.counterX; x < l.counterX+l.counterW; x++ {
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
	h.Spin(Draw(types, len(h.Rooms), rng))
	for i := range h.Rooms {
		h.Land(i)
	}

	h.Spawn = image.Pt((entrance+1)*TileSize, (l.street+1)*TileSize+TileSize/2) // on the Entrance's center line
	h.Archway = image.Pt((archway+archwayWidth/2)*TileSize, (l.hall+1)*TileSize)
	h.ConciergeStart = cellCenter(computer, l.counter-2)
	h.LaneEntry = cellCenter(archway+archwayWidth-1, l.counter+2)
	for i := range h.Rooms { // the first to leave pays farthest east (D62)
		h.Rooms[i].Pay = cellCenter(l.counterX+len(h.Rooms)-1-i, l.counter+1)
	}
	h.Terminal = image.Rect((computer-1)*TileSize, (l.counter-2)*TileSize, (computer+2)*TileSize, (l.counter+2)*TileSize)
	return h, nil
}

// Draw picks each Room's next Room Type: independent, equal odds per type name, repeats allowed (D46),
// then one of that type's variants, equal odds (D65).
func Draw(types []RoomType, n int, rng *rand.Rand) []RoomType {
	var names []string
	for _, t := range types {
		if !slices.Contains(names, t.Name) {
			names = append(names, t.Name)
		}
	}
	next := make([]RoomType, n)
	for i := range next {
		name := names[rng.IntN(len(names))]
		vs := slices.DeleteFunc(slices.Clone(types), func(t RoomType) bool { return t.Name != name })
		next[i] = vs[rng.IntN(len(vs))]
	}
	return next
}

// Spin gives every Room its next Room Type (D47), one per Room. Type, Rate and Stay Spot change at once
// (D49); the cells follow when each Room Lands, at its reel stop (D50).
func (h *Hotel) Spin(next []RoomType) {
	for i := range h.Rooms {
		r := &h.Rooms[i]
		r.Type, r.Stay = next[i], r.Origin.Mul(TileSize).Add(next[i].Stay)
	}
}

// Land stamps Room i's cells, tiles and collisions, from its Type, under the hotel's outer side lines.
// Doors never move (D45), so the Doors and every anchor stay valid.
func (h *Hotel) Land(i int) {
	r := h.Rooms[i]
	for layer, cells := range r.Type.Tiles {
		for c, tid := range cells {
			if x := r.Origin.X + c%RoomW; layer != LayerDecor || !h.OuterLine(x) {
				h.Map.Set(layer, x, r.Origin.Y+c/RoomW, tid)
			}
		}
	}
}

// OuterLine: column x carries a hotel outer side line in its decor layer. It stays still during a Spin.
func (h *Hotel) OuterLine(x int) bool { return x == 1 || x == h.l.w-2 }

// Checkout is Room i's own lane out (D62): from its Stay Spot to its Pay Spot, then on to the Street.
// Below the counter the Lobby has 3 rows: pay, approach, exit. A guest comes down the Archway's east
// column, walks east along the approach row and steps up into its Pay Spot; after paying it steps down
// to the exit row and walks west to the Entrance. Guests leave in Room order and Room 0 pays farthest
// east, so a guest never walks past one still ahead of it: the lanes never meet head-on. The game
// releases them so they reach LaneEntry in that order, apart: lanes from both sides merge above it.
func (h *Hotel) Checkout(i int) (toPay, toStreet []image.Point) {
	l, r := h.l, h.Rooms[i]
	pay := r.Pay.Div(TileSize)
	via := func(own int, pts ...image.Point) []image.Point {
		var path []image.Point
		for k := 1; k < len(pts); k++ {
			path = append(path, h.Path(pts[k-1], pts[k], own)...)
		}
		return path
	}
	approach, exit := l.counter+2, l.counter+3
	toPay = via(i, r.Stay, h.LaneEntry, cellCenter(pay.X, approach), r.Pay)
	toStreet = via(-1, r.Pay, cellCenter(pay.X, exit), cellCenter(l.entrance+1, exit), h.Spawn)
	return toPay, toStreet
}
