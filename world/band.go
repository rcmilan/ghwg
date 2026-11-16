package world

import (
	"image"
	"image/color"
	"strings"
)

// The top band (07 F1, 2026-11-14): the strip where a wall meets the ceiling. Every wall has the same one, and
// where walls meet it is one shape: its outline runs along its outer edge only, never across a join. So it is
// not tile art: Bands draws it from where the walls stand, and the game draws each cell's part over its tile.
var (
	BandColor    = color.RGBA{0x6b, 0x54, 0x4a, 0xff} // dark wood; just bright enough that the ghost palette keeps it black, the outline blue
	BandOutline  = color.RGBA{0x3a, 0x3a, 0x50, 0xff}
	bandW        = 6 // px: a side line's width, a face's band height, the outline included
	bandFace     = image.Rect(0, 0, TileSize, bandW)
	bandWestLine = image.Rect(0, 0, bandW, TileSize)
	bandEastLine = image.Rect(TileSize-bandW, 0, TileSize, TileSize)
)

// BandRect: the part of the cell that a tile's top band takes, in cell px; empty for a tile without one.
func BandRect(name string) image.Rectangle {
	line, east := bandWestLine, strings.HasPrefix(name, "border_east")
	if east {
		line = bandEastLine
	}
	switch {
	case name == "wall_lintel", name == "border_bottom":
		return bandFace
	case name == "wall_stub_top": // the kitchen wall's end: the east line's last 6 px
		return bandEastLine.Intersect(bandFace)
	case name == "wall_halfw_top": // half a face (H1): the half the wall stands in
		return bandFace.Intersect(image.Rect(TileSize/2, 0, TileSize, TileSize))
	case name == "wall_halfe_top":
		return bandFace.Intersect(image.Rect(0, 0, TileSize/2, TileSize))
	case strings.HasPrefix(name, "wall_") && strings.HasSuffix(name, "_top"):
		return bandFace
	case east || strings.HasPrefix(name, "border_west"):
		if strings.HasSuffix(name, "_corner") { // the line ends on a face
			return line.Intersect(bandFace)
		}
		return line
	}
	return image.Rectangle{}
}

// BandMask is the walls' top bands, the map at 1 px per world px, one byte a px: 0 none, 1 band (BandColor),
// 2 its outline (BandOutline), on the px that touch anything else, corners included. A growth computes it in its
// first frame (08 W3: as RGBA scanned over the whole map it took 42 ms as wasm).
type BandMask struct {
	W, H int
	Pix  []uint8
}

// At is the mask's px at (x, y); 0 off the map.
func (b *BandMask) At(x, y int) uint8 {
	if x < 0 || y < 0 || x >= b.W || y >= b.H {
		return 0
	}
	return b.Pix[y*b.W+x]
}

func (m *Tilemap) BandMask() *BandMask {
	b := &BandMask{W: m.W * TileSize, H: m.H * TileSize}
	b.Pix = make([]uint8, b.W*b.H)
	var rs []image.Rectangle
	for l := LayerFloor + 1; l < NumLayers; l++ {
		for y := range m.H {
			for x := range m.W {
				r := BandRect(m.TS.Tiles[m.At(l, x, y)].Name).Add(m.Origin(x, y)).Intersect(image.Rect(0, 0, b.W, b.H))
				if r.Empty() {
					continue
				}
				rs = append(rs, r)
				for py := r.Min.Y; py < r.Max.Y; py++ {
					row := b.Pix[py*b.W:]
					for px := r.Min.X; px < r.Max.X; px++ {
						row[px] = 1
					}
				}
			}
		}
	}
	for _, r := range rs { // an outline px stays non-zero, so marking it never hides a neighbour's band
		for y := r.Min.Y; y < r.Max.Y; y++ {
			for x := r.Min.X; x < r.Max.X; x++ {
				if x > r.Min.X && x < r.Max.X-1 && y > r.Min.Y && y < r.Max.Y-1 {
					continue // inside its own band: every neighbour is band
				}
				for d := range 9 {
					if b.At(x+d%3-1, y+d/3-1) == 0 {
						b.Pix[y*b.W+x] = 2
						break
					}
				}
			}
		}
	}
	return b
}

// Bands is BandMask in colors.
func (m *Tilemap) Bands() *image.RGBA {
	b := m.BandMask()
	img := image.NewRGBA(image.Rect(0, 0, b.W, b.H))
	for i, v := range b.Pix {
		if v != 0 {
			c := [...]color.RGBA{1: BandColor, 2: BandOutline}[v]
			copy(img.Pix[4*i:], []uint8{c.R, c.G, c.B, c.A})
		}
	}
	return img
}
