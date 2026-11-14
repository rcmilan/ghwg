package world

import (
	"errors"
	"fmt"
	"image"
	"slices"
	"strconv"
	"strings"
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
	KitchenW     = 6 // the kitchen's prop row (07 F1): the cells of its back wall next to its west wall
	doorwayRow   = 4 // the kitchen's open doorway, in its west wall: this row below the Hall Wall's top, and the next
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
	counterX   int // counter_left: the counter ends just west of the Archway
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
	l.counterX = l.archway - counterW
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
	Spots  []Spot            // activity spots, world px (D168)
	Mirror image.Rectangle   // where SPACE shifts at its in-Room Mirror, world px; empty without one (K1)
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
	MirrorSpot     image.Rectangle // where SPACE shifts: the cell in front of the mirror and the one east of it, clear of the Terminal
	Lights         []image.Point   // ceiling lights over the Corridors (D95)
	Lobby          image.Rectangle // the Lobby, Hall Wall face included: lit as a whole at night (S6 review)
	Stock          image.Rectangle // the kitchen's prop row, flush against its back wall: solid, its props fetched from the row below (E8, K1, F1)
	Kitchen        image.Rectangle // the kitchen (F1): the Lobby's part east of the Archway, walled off, its back wall's face included
}

// cellCenter is the world-px center of cell (x,y).
func cellCenter(x, y int) image.Point { return image.Pt(x*TileSize+TileSize/2, y*TileSize+TileSize/2) }

// Build stamps the whole hotel for a cols x rows grid of Rooms, each a Standard Single (K1: the Run-start
// draw is gone, so every Run starts the same).
func Build(ts *Tileset, types []RoomType, cols, rows int) (*Hotel, error) {
	start, ok := TypeOf(types, "Single")
	if !ok {
		return nil, errors.New("world: no Standard Single room type")
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
	kx := l.archway + l.archwayW // the kitchen's west wall line: the kitchen fills the Lobby east of the Archway (F1)
	put := func(l, x, y int, name string) { m.Set(l, x, y, id(name)) }
	// face: a 2-row wall face over cols x0..x1; gaps are openings (lintel on top, floor below).
	skin := "wall"
	face := func(x0, x1, y int, gaps ...int) {
		in := func(x int) bool { return slices.Contains(gaps, x) }
		for x := x0; x <= x1; x++ {
			if in(x) {
				put(LayerWalls, x, y, "wall_lintel")
				continue
			}
			left, right := x == x0 || in(x-1), x == x1 || in(x+1)
			kind := skin + "_mid"
			switch {
			case left && right:
				kind = skin + "_single"
			case left:
				kind = skin + "_left"
			case right:
				kind = skin + "_right"
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
			case x > kx && y >= l.hall && y < l.front: // the kitchen, the sheet's shadow cells under its back wall and beside its west one
				n = "floor_kitchen"
				if north, west := y == l.hall+2, x == kx+1; north && west {
					n += "_nw"
				} else if north {
					n += "_n"
				} else if west {
					n += "_w"
				}
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
	face(1, kx, l.hall, gaps...) // Hall Wall + Lobby Archway
	skin = "wall_kitchen"
	face(kx+1, l.w-2, l.hall) // the kitchen's back wall: the Hall Wall's east part
	skin = "wall"
	face(1, l.w-2, l.front, entrance, entrance+1) // exterior face + Entrance
	for i, n := range []string{"entrance_left", "entrance_right"} {
		put(LayerDecor, entrance+i, l.front, n+"_top")
		put(LayerDecor, entrance+i, l.front+1, n+"_bottom")
	}
	// front windows (D67, S2 review): 2 cells wide, 2 per side, mirrored about the Entrance and evenly
	// spaced along the wall between the side line (x=1) and the Entrance; a spare cell widens the middle gap
	// F1: the east pair stands evenly on the kitchen's part of the wall (x=kx+1..w-3), so none straddles its west wall
	edge := (l.w - 3 - kx - 2*2) / 3 // free wall cells between a window and the kitchen's side walls
	for _, w := range []int{kx + 1 + edge, l.w - 4 - edge} {
		for _, x := range []int{w, 2*entrance - w} {
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
	computer := l.counterX + 1 // away from the Archway
	for x := l.counterX; x < l.counterX+counterW; x++ {
		n := "counter_mid"
		switch x {
		case l.counterX:
			n = "counter_left"
		case computer:
			n = "counter_computer_bottom"
		case l.counterX + counterW - 1:
			n = "counter_right"
		}
		put(LayerDecor, x, l.counter, n)
	}
	put(LayerDecor, computer, l.counter-1, "counter_computer_top")
	mirror := archway - 2 // on the Hall Wall face, 2 cells west of the Archway, behind the counter (D142, F1)
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
	// the kitchen's west wall, its doorway open: the part above the doorway ends on a face of its own, in the
	// hotel's wallpaper and as wide as the line, as every side line ends on a face
	put(LayerDecor, kx, l.hall, "border_east_top")
	for y := l.hall + 1; y < l.front; y++ {
		switch d := y - l.hall - doorwayRow; {
		case d == -2:
			put(LayerDecor, kx, y, "border_east_corner")
			put(LayerWalls, kx, y, "wall_stub_top")
			put(LayerWalls, kx, y+1, "wall_stub_bottom")
		case d < -2 || d > 1:
			put(LayerDecor, kx, y, "border_east")
		}
	}
	// it ends on the front face as the outer lines do: the face's top row is walkable (D66), and without this
	// one walked round the wall's end, behind the face (live test, 2026-11-14)
	put(LayerDecor, kx, l.front, "border_east_corner")
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
	for i := range h.Rooms {
		h.Land(i, start, false)
	}

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
	h.MirrorSpot = image.Rect(mirror*TileSize, (l.hall+2)*TileSize, (mirror+2)*TileSize, (l.hall+3)*TileSize)
	h.Kitchen = image.Rect((kx+1)*TileSize, l.hall*TileSize, (l.w-1)*TileSize, l.front*TileSize)
	h.Stock = image.Rect((kx+1)*TileSize, (l.hall+2)*TileSize, (kx+1+KitchenW)*TileSize, (l.hall+3)*TileSize)
	m.Block = map[image.Point]bool{}
	for x := kx + 1; x <= kx+KitchenW; x++ { // nobody walks over the props (K1, 2026-11-11)
		m.Block[image.Pt(x, l.hall+2)] = true
	}
	return h, nil
}

// TypeOf: the Standard Room Type of a bed layout (the first of its variants), what a Room opens as or
// changes to (K1: the Deluxe tier is a later unlock).
func TypeOf(types []RoomType, beds string) (RoomType, bool) {
	i := slices.IndexFunc(types, func(t RoomType) bool { return t.Name == "Standard" && t.Beds == beds })
	if i < 0 {
		return RoomType{}, false
	}
	return types[i], true
}

// Furnish gives every Room its Room Type, one per Room, with no Mirror (Land).
func (h *Hotel) Furnish(types []RoomType) {
	for i := range h.Rooms {
		h.Land(i, types[i], false)
	}
}

const mirrorTile = "prop_mirror_" // the Mirror's two tiles: r0c0 over r1c0

// Land gives Room i its Room Type and stamps its cells, tiles and collisions under the hotel's outer side
// lines. Doors never move (D45), so the Doors and every anchor stay valid. With mirror the Room has its
// in-Room Mirror (K1): the type's own wall mirror if it has one, else one hung in the middle of its back
// wall; without, a type's wall mirror is left off, and nobody looks in it.
func (h *Hotel) Land(i int, t RoomType, mirror bool) {
	r, ts := &h.Rooms[i], h.Map.TS
	o := r.Origin.Mul(TileSize)
	r.Type, r.Stays, r.Lanes, r.Spots, r.Mirror = t, nil, nil, nil, image.Rectangle{}
	for _, s := range t.Spots {
		if mirror || s.Verb != "look in" {
			r.Spots = append(r.Spots, Spot{s.At.Add(o), s.Prop.Add(o), s.Verb})
		}
	}
	for _, p := range t.Stays {
		r.Stays = append(r.Stays, o.Add(p))
	}
	for _, b := range t.Lanes {
		r.Lanes = append(r.Lanes, b.Add(o))
	}
	glass := image.Pt(RoomW/2, 1) // the Mirror's lower tile, room-local cell
	for layer, cells := range t.Tiles {
		for c, tid := range cells {
			if name := ts.Tiles[tid].Name; strings.HasPrefix(name, mirrorTile) {
				if !mirror {
					tid = 0
				} else if strings.HasPrefix(name, mirrorTile+"r1") {
					glass = image.Pt(c%RoomW, c/RoomW)
				}
			}
			if x := r.Origin.X + c%RoomW; layer != LayerDecor || !h.OuterLine(x) {
				h.Map.Set(layer, x, r.Origin.Y+c/RoomW, tid)
			}
		}
	}
	if !mirror {
		return
	}
	at := r.Origin.Add(glass)
	for dy, name := range [...]string{mirrorTile + "r1c0", mirrorTile + "r0c0"} {
		id, _ := ts.ID(name)
		h.Map.Set(LayerProps, at.X, at.Y-dy, id)
	}
	r.Mirror = image.Rectangle{at.Add(image.Pt(-1, 1)).Mul(TileSize), at.Add(image.Pt(2, 2)).Mul(TileSize)} // the cells below it, as the Lobby's MirrorSpot
}

// OuterLine: column x carries a hotel outer side line in its decor layer.
func (h *Hotel) OuterLine(x int) bool { return x == 1 || x == h.l.w-2 }

// Grid is the hotel's Room grid, cols × rows.
func (h *Hotel) Grid() (cols, rows int) { return h.l.cols, h.l.rows }
