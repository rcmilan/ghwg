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
	Cols, Rows   = 2, 2
	corridor     = 2 // Corridor width: between Room columns, between Room rows and above the Hall Wall (D55)
	lobbyRows    = 6 // floor rows between the Hall Wall and the front face (D56)
	behindRows   = 2 // Lobby rows behind the counter (D40)
	streetRows   = 3
	counterX     = 3 // counter_left; the counter runs counterX..counterX+4, the computer at counterX+3
	archwayWidth = 4
)

// layout is the hotel's geometry in map cells, derived from the grid (context/02-rooms/index.md D54-D57).
type layout struct {
	cols, rows int
	w, h       int
	hall       int // Hall Wall face (2 rows); the Lobby Archway is centered in it
	counter    int
	front      int // exterior south face (2 rows); the Entrance is centered in it
	street     int
}

func newLayout(cols, rows int) layout {
	l := layout{cols: cols, rows: rows, w: 2 + cols*RoomW + (cols-1)*corridor} // +2: the void margin columns
	l.hall = 1 + rows*(RoomH+corridor)
	l.counter = l.hall + 2 + behindRows
	l.front = l.hall + 2 + lobbyRows
	l.street = l.front + 2
	l.h = l.street + streetRows
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
	PaySpot        image.Point     // in front of the computer
	PayExit        image.Point     // below the pay spot on the Lobby's last row: paid guests leave via here (D59)
	Terminal       image.Rectangle // the computer and the cell on either side of it, both sides of the counter
}

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

	archway, entrance := l.w/2-archwayWidth/2, l.w/2-1 // centered (D55); one cell left of center on odd widths
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
	computer := counterX + 3
	for i, n := range []string{"counter_left", "counter_mid", "counter_mid", "counter_computer_bottom", "counter_right"} {
		put(LayerDecor, counterX+i, l.counter, n)
	}
	put(LayerDecor, computer, l.counter-1, "counter_computer_top")
	for _, n := range []string{"border_west_top", "border_west", "border_west_corner", "border_east_top", "border_east", "border_east_corner"} {
		id(n) // checked here so Spin's outer lines never miss
	}
	for i, r := range h.Rooms {
		top := r.Door.Mul(TileSize)
		h.Doors = append(h.Doors, Door{Room: i, Rect: image.Rectangle{top, top.Add(image.Pt(TileSize, 2*TileSize))}})
	}
	h.Doors = append(h.Doors, Door{Room: -1, Rect: image.Rect(entrance*TileSize, l.front*TileSize, (entrance+2)*TileSize, (l.front+2)*TileSize)})
	if len(missing) > 0 {
		return nil, fmt.Errorf("world: tileset lacks tiles %v", missing)
	}
	h.Spin(Draw(types, len(h.Rooms), rng))

	center := func(x, y int) image.Point { return image.Pt(x*TileSize+TileSize/2, y*TileSize+TileSize/2) }
	h.Spawn = image.Pt((entrance+1)*TileSize, (l.street+1)*TileSize+TileSize/2) // on the Entrance's center line
	h.Archway = image.Pt((archway+archwayWidth/2)*TileSize, (l.hall+1)*TileSize)
	h.ConciergeStart = center(computer, l.counter-2)
	h.PaySpot = center(computer, l.counter+1)
	h.PayExit = center(computer, l.front-1)
	h.Terminal = image.Rect((computer-1)*TileSize, (l.counter-2)*TileSize, (computer+2)*TileSize, (l.counter+2)*TileSize)
	return h, nil
}

// Draw picks each Room's next Room Type: independent, equal odds, repeats allowed (D46).
func Draw(types []RoomType, n int, rng *rand.Rand) []RoomType {
	next := make([]RoomType, n)
	for i := range next {
		next[i] = types[rng.IntN(len(types))]
	}
	return next
}

// Spin gives every Room its next Room Type (D47), one per Room, and re-stamps their cells. Doors never
// move (D45), so the Doors and every anchor stay valid.
func (h *Hotel) Spin(next []RoomType) {
	for i := range h.Rooms {
		r, t := &h.Rooms[i], next[i]
		for layer, cells := range t.Tiles {
			for c, tid := range cells {
				h.Map.Set(layer, r.Origin.X+c%RoomW, r.Origin.Y+c/RoomW, tid)
			}
		}
		r.Type, r.Stay = t, r.Origin.Mul(TileSize).Add(t.Stay)
	}
	// hotel outer side walls: one unbroken line over every wall that meets them (overwrites the rooms' own)
	for x, side := range map[int]string{1: "west", h.l.w - 2: "east"} {
		id := func(n string) TileID { t, _ := h.Map.TS.ID("border_" + side + n); return t } // checked by Build
		h.Map.Set(LayerDecor, x, 1, id("_top"))
		for y := 2; y < h.l.front; y++ {
			h.Map.Set(LayerDecor, x, y, id(""))
		}
		h.Map.Set(LayerDecor, x, h.l.front, id("_corner"))
	}
}
