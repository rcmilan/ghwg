package world

import "image"

// Tilemap is a grid of tiles in NumLayers layers. Collision in a cell is the union of its layers' rects.
type Tilemap struct {
	W, H  int
	Cells [NumLayers][]TileID // W*H each, row-major
	TS    *Tileset
}

func NewTilemap(ts *Tileset, w, h int) *Tilemap {
	m := &Tilemap{W: w, H: h, TS: ts}
	for l := range m.Cells {
		m.Cells[l] = make([]TileID, w*h)
	}
	return m
}

func (m *Tilemap) inside(x, y int) bool { return x >= 0 && y >= 0 && x < m.W && y < m.H }

// At returns the tile at cell (x,y) of layer l; 0 when empty or outside the map.
func (m *Tilemap) At(l, x, y int) TileID {
	if !m.inside(x, y) {
		return 0
	}
	return m.Cells[l][y*m.W+x]
}

func (m *Tilemap) Set(l, x, y int, id TileID) { m.Cells[l][y*m.W+x] = id }

// Solid reports whether any layer has a collision rect in cell (x,y). Outside the map is solid.
func (m *Tilemap) Solid(x, y int) bool {
	if !m.inside(x, y) {
		return true
	}
	for l := range m.Cells {
		if len(m.TS.Tiles[m.At(l, x, y)].Solid) > 0 {
			return true
		}
	}
	return false
}

// BlockedBox reports whether the world-px box r overlaps any collision rect or leaves the map.
// Door tiles carry no rect: door state is handled by the caller.
func (m *Tilemap) BlockedBox(r image.Rectangle) bool {
	if !r.In(image.Rect(0, 0, m.W*TileSize, m.H*TileSize)) {
		return true
	}
	for y := r.Min.Y / TileSize; y <= (r.Max.Y-1)/TileSize; y++ {
		for x := r.Min.X / TileSize; x <= (r.Max.X-1)/TileSize; x++ {
			origin := image.Pt(x*TileSize, y*TileSize)
			for l := range m.Cells {
				for _, s := range m.TS.Tiles[m.At(l, x, y)].Solid {
					if s.Add(origin).Overlaps(r) {
						return true
					}
				}
			}
		}
	}
	return false
}
