package game

import (
	"image"

	"shift/world"
)

// atPanel: the Room whose Door Panel the Concierge stands at (04 D113), -1 if none.
func (g *Game) atPanel() int {
	at := image.Pt(int(g.player.X), int(g.player.Y)).Div(world.TileSize)
	for i, r := range g.hotel.Rooms {
		if r.PanelSpot().Div(world.TileSize) == at {
			return i
		}
	}
	return -1
}
