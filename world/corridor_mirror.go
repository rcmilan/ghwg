package world

import "image"

// A Corridor Mirror (08 B1) hangs on a Room's front wall, near a corner, over its Corridor: one for each
// Corridor a New floor made. Covered until Open; the game decides which Room, which corner and when.
type CorridorMirror struct {
	Floor int             // above the ground floor: 1 is the first a growth made
	Col   int             // the Room column whose front wall carries it
	West  bool            // at the Room's west corner, else its east one
	Open  bool            // uncovered
	Cell  image.Point     // cell of its upper tile; the lower is the next one down
	Glass image.Rectangle // world px: the two cells it covers
	Spot  image.Rectangle // world px: the Corridor cell below it, where SPACE acts
	Room  int             // the Room whose wall it is
}

// corridorCol: the Mirror's column in its Room, a cell clear of the end plants and the doormat.
func corridorCol(west bool) int {
	if west {
		return 2
	}
	return RoomW - 3
}

// SetCorridorMirrors hangs the Mirrors, replacing any hung before; each is given by Floor, Col, West and Open.
// A Floor or Col outside the grid is dropped.
func (h *Hotel) SetCorridorMirrors(ms []CorridorMirror) {
	h.Corridors = h.Corridors[:0:0]
	cols, rows := h.Grid()
	for _, m := range ms {
		r := rows - 1 - m.Floor
		if m.Floor < 1 || r < 0 || m.Col < 0 || m.Col >= cols {
			continue
		}
		m.Room = r*cols + m.Col
		m.Cell = h.Rooms[m.Room].Origin.Add(image.Pt(corridorCol(m.West), doorRow))
		m.Glass = image.Rectangle{m.Cell.Mul(TileSize), m.Cell.Add(image.Pt(1, 2)).Mul(TileSize)}
		m.Spot = image.Rectangle{m.Cell.Add(image.Pt(0, 2)).Mul(TileSize), m.Cell.Add(image.Pt(1, 3)).Mul(TileSize)}
		h.Corridors = append(h.Corridors, m)
		h.hangCorridorMirrors(m.Room)
	}
}

// hangCorridorMirrors stamps the Mirrors on Room i's front wall (Land wipes it).
func (h *Hotel) hangCorridorMirrors(i int) {
	for _, m := range h.Corridors {
		if m.Room != i {
			continue
		}
		for dy, name := range [...]string{mirrorTile + "r0c0", mirrorTile + "r1c0"} {
			id, _ := h.Map.TS.ID(name)
			h.Map.Set(LayerDecor, m.Cell.X, m.Cell.Y+dy, id)
		}
	}
}
