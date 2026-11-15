package world

import "testing"

// F1: the top band is seamless. No outline px lies between two band px, across or down, so no line cuts a
// join and no two walls stand a px apart; and every px of a band tile's own band is drawn.
func TestBandsSeamless(t *testing.T) {
	forGrids(t, func(t *testing.T, h *Hotel, l layout) {
		img := h.Map.Bands()
		band := func(x, y int) bool { return img.RGBAAt(x, y) == BandColor }
		n := 0
		for y := 1; y < img.Rect.Dy()-1; y++ {
			for x := 1; x < img.Rect.Dx()-1; x++ {
				if img.RGBAAt(x, y) != BandOutline {
					continue
				}
				n++
				if band(x-1, y) && band(x+1, y) || band(x, y-1) && band(x, y+1) {
					t.Fatalf("an outline px at (%d,%d) cuts across the band", x, y)
				}
			}
		}
		if n == 0 {
			t.Fatal("no band drawn")
		}
		if x, y := (l.archway+l.archwayW)*TileSize+TileSize-3, l.hall*TileSize+3; !band(x, y) { // the kitchen wall's line, through the Hall Wall
			t.Errorf("no band at (%d,%d), where the kitchen's wall meets the Hall Wall", x, y)
		}
	})
}

// The ghost palette draws a maze px blue when its luminance is 0.33 or less (game/ghost.go): the band stays
// above it, so only the outline is blue.
func TestBandGhostBlack(t *testing.T) {
	c := BandColor
	if l := (0.299*float64(c.R) + 0.587*float64(c.G) + 0.114*float64(c.B)) / 255; l <= 0.33 {
		t.Errorf("the band's luminance is %.3f: the ghost dimension would fill the walls blue", l)
	}
}
