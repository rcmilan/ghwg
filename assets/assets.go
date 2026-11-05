// Package assets contains the presentation palette and, later, sprite-sheet definitions.
// Keeping this boundary lets the LimeZu tiles replace the placeholder art without touching
// the game rules or entities.
package assets

import (
	"embed"
	"image/color"
)

// FS holds the Tiled data: tiles/*.png + tiles/hotel.tsj (tileset) and rooms/*.tmj (Room Types),
// plus the LimeZu character sheets: characters/<Name>_{idle_anim,run}_16x16.png (D26).
//
//go:embed tiles rooms characters
var FS embed.FS

var (
	Night     = color.RGBA{18, 22, 35, 255}
	Wall      = color.RGBA{44, 49, 69, 255}
	Floor     = color.RGBA{78, 83, 100, 255}
	FloorLine = color.RGBA{63, 68, 84, 255}
	Accent    = color.RGBA{91, 217, 186, 255}
	Pale      = color.RGBA{199, 239, 255, 255}
	Guest     = color.RGBA{255, 184, 113, 255}
	Ink       = color.RGBA{15, 17, 28, 255}
	Locked    = color.RGBA{208, 91, 107, 255}
	Money     = color.RGBA{247, 216, 101, 255}
)
