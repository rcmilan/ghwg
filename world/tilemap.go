package world

import (
	"image"
	"strings"
)

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

// SortY is the world-px y at which cell (x,y) is drawn among the actors (D11): the lower edge of its
// footprint, `base` cells down (the largest base among its layers' tiles). An actor whose feet are above
// it is behind the cell's tiles. Props are their own objects and keep out of it (see TileSortY).
func (m *Tilemap) SortY(x, y int) int {
	base := 0
	for l := range LayerProps {
		base = max(base, m.TS.Tiles[m.At(l, x, y)].Base)
	}
	return (y + base + 1) * TileSize
}

// TileSortY is SortY for the tile in layer l of cell (x,y). A side line (`border_*`) is one long wall,
// so all its cells sort at its south end, where its run of side-line cells ends. Otherwise an actor
// walking beside it would be split: in front of the cells above its feet, behind the one at its feet.
// A flat tile (a rug) sorts at 0, under every actor: an actor's sprite reaches below its feet, so even one
// standing just above a rug's cell overlaps it. Any other prop sorts by its own base only: a bed standing
// against the back wall must not drag the window above it down to the bed's bottom row, or an actor
// walking up to the window is drawn behind the curtain.
func (m *Tilemap) TileSortY(l, x, y int) int {
	if t := m.TS.Tiles[m.At(l, x, y)]; t.Flat {
		return 0
	} else if l == LayerProps {
		return (y + t.Base + 1) * TileSize
	}
	if !m.sideLine(l, x, y) {
		return m.SortY(x, y)
	}
	for m.sideLine(l, x, y+1) {
		y++
	}
	return m.SortY(x, y)
}

func (m *Tilemap) sideLine(l, x, y int) bool {
	return strings.HasPrefix(m.TS.Tiles[m.At(l, x, y)].Name, "border_")
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

// Wall: the cell holds a wall (a solid wall face or a side line), which blocks sight; furniture doesn't.
func (m *Tilemap) Wall(x, y int) bool {
	return len(m.TS.Tiles[m.At(LayerWalls, x, y)].Solid) > 0 || m.sideLine(LayerDecor, x, y) || m.sideLine(LayerWalls, x, y)
}
