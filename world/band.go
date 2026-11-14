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

// Bands draws every wall's top band, the map at 1 px per world px: BandColor, and BandOutline on the px that
// touch anything else, corners included.
func (m *Tilemap) Bands() *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, m.W*TileSize, m.H*TileSize))
	for l := LayerFloor + 1; l < NumLayers; l++ {
		for y := range m.H {
			for x := range m.W {
				r := BandRect(m.TS.Tiles[m.At(l, x, y)].Name).Add(image.Pt(x, y).Mul(TileSize))
				for py := r.Min.Y; py < r.Max.Y; py++ {
					for px := r.Min.X; px < r.Max.X; px++ {
						img.SetRGBA(px, py, BandColor)
					}
				}
			}
		}
	}
	in := func(x, y int) bool { return image.Pt(x, y).In(img.Rect) && img.RGBAAt(x, y).A != 0 }
	var edge []image.Point
	for y := range img.Rect.Dy() {
		for x := range img.Rect.Dx() {
			if !in(x, y) {
				continue
			}
			for d := range 9 {
				if !in(x+d%3-1, y+d/3-1) {
					edge = append(edge, image.Pt(x, y))
					break
				}
			}
		}
	}
	for _, p := range edge {
		img.SetRGBA(p.X, p.Y, BandOutline)
	}
	return img
}
