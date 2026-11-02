package world

import (
	"errors"
	"fmt"
	"image"
	"math/rand/v2"
	"slices"
	"strconv"
)

// Hotel layout, in map cells (context/01-tileset/index.md -> "Hotel layout").
const (
	MapW       = 32
	MapH       = 38
	hallRow    = 23 // Hall Wall face; Lobby Archway at cols 14-17
	frontRow   = 33 // exterior south face; Entrance at cols 15-16
	streetRow  = 35
	counterRow = 28
)

var roomOrigins = [...]image.Point{{1, 1}, {18, 1}, {1, 12}, {18, 12}} // Rooms 101-104

// Room is one of the hotel's guest rooms (101-104), built from a Room Type.
type Room struct {
	Number string
	Type   RoomType
	Origin image.Point // cell of the room's top-left corner
	Door   image.Point // cell of the door's top half; the bottom half is Door+(0,1)
	Stay   image.Point // Stay Spot, world px
}

// Hotel is the built map plus the anchors the game needs. Points are world px.
type Hotel struct {
	Map            *Tilemap
	Rooms          []Room
	Spawn          image.Point     // on the street below the Entrance; guests also leave here
	ConciergeStart image.Point     // behind the counter's computer
	PaySpot        image.Point     // in front of the computer
	Terminal       image.Rectangle // the computer and the cell on either side of it, both sides of the counter
}

// Build stamps the whole hotel. Each Room gets a Room Type drawn from types with rng (repeats allowed).
func Build(ts *Tileset, types []RoomType, rng *rand.Rand) (*Hotel, error) {
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
	m := NewTilemap(ts, MapW, MapH)
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

	for y := range MapH {
		for x := range MapW {
			n := "floor_clay"
			switch {
			case y >= streetRow:
				n = "floor_street" // full width, margin included
			case y == 0 || x == 0 || x == MapW-1:
				n = "void"
			}
			put(LayerFloor, x, y, n)
		}
	}
	face(14, 17, 1) // alcove: hotel north wall between 101 and 102

	h := &Hotel{Map: m}
	for i, o := range roomOrigins {
		t := types[rng.IntN(len(types))]
		for l, cells := range t.Tiles {
			for c, tid := range cells {
				if tid != 0 {
					m.Set(l, o.X+c%RoomW, o.Y+c/RoomW, tid)
				}
			}
		}
		h.Rooms = append(h.Rooms, Room{
			Number: strconv.Itoa(101 + i),
			Type:   t,
			Origin: o,
			Door:   o.Add(image.Pt(t.DoorCol, doorRow)),
			Stay:   o.Mul(TileSize).Add(t.Stay),
		})
	}

	face(1, 30, hallRow, 14, 15, 16, 17) // Hall Wall + Lobby Archway
	face(1, 30, frontRow, 15, 16)        // exterior face + Entrance
	for x, n := range map[int]string{15: "entrance_left", 16: "entrance_right"} {
		put(LayerDecor, x, frontRow, n+"_top")
		put(LayerDecor, x, frontRow+1, n+"_bottom")
	}
	for x, n := range map[int]string{3: "counter_left", 4: "counter_mid", 5: "counter_mid", 6: "counter_computer_bottom", 7: "counter_right"} {
		put(LayerDecor, x, counterRow, n)
	}
	put(LayerDecor, 6, counterRow-1, "counter_computer_top")
	// hotel outer side walls: one unbroken line over every wall that meets them (overwrites the rooms' own)
	for x, side := range map[int]string{1: "west", MapW - 2: "east"} {
		put(LayerDecor, x, 1, "border_"+side+"_top")
		for y := 2; y < frontRow; y++ {
			put(LayerDecor, x, y, "border_"+side)
		}
		put(LayerDecor, x, frontRow, "border_"+side+"_corner")
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("world: tileset lacks tiles %v", missing)
	}

	center := func(x, y int) image.Point { return image.Pt(x*TileSize+TileSize/2, y*TileSize+TileSize/2) }
	h.Spawn = image.Pt(MapW/2*TileSize, (streetRow+1)*TileSize+TileSize/2) // on the center line
	h.ConciergeStart = center(6, counterRow-2)
	h.PaySpot = center(6, counterRow+1)
	h.Terminal = image.Rect(5*TileSize, (counterRow-2)*TileSize, 8*TileSize, (counterRow+2)*TileSize)
	return h, nil
}
