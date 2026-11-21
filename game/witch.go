package game

import (
	"image"
	"maps"
	"strconv"

	"ghosthotel/entities"
	"ghosthotel/world"
)

// 08 W5: Magenta's own sheets, Amelia's under a witch hat (tools/witchhat). She is a base character but
// not one of guestCast: never a random walk-in, a passer-by or a Shadow.
const witchName = nameMagenta

// witchHat: the hat's own colors, kept as drawn under any look (its outlines are the sheet's, kept too).
var witchHat = [...]uint32{0x4d4263, 0x5a4d75, 0x9f74a8, 0x805e8e}

func init() {
	pal := maps.Clone(palettes["Amelia"])
	for _, c := range witchHat {
		pal[c] = [2]region{keep, keep}
	}
	palettes[witchName], ghostTints[witchName] = pal, ghostTints["Amelia"]
}

func (g *Game) loadWitch() error {
	b, err := loadCharacters(witchName)
	if err != nil {
		return err
	}
	g.bases[witchName] = b[witchName]
	g.characters[witchName] = dress(b[witchName], palettes[witchName], look{})
	return nil
}

// DevWitch: the Concierge on the Street at the Entrance, four Magentas beside him, each in a look of her
// own, facing right, up, left and (the one next to him) down (with -shot -at).
func (g *Game) DevWitch() {
	at := g.hotel.Spawn
	g.player.X, g.player.Y = float64(at.X), float64(at.Y)
	w, h := g.mapSize()
	g.cam.Snap(g.player.X, g.player.Y, w, h)
	for f := entities.FaceRight; f <= entities.FaceDown; f++ {
		p := &passer{passed: true, rolled: true} // a passer-by that stands: drawn by the Street, no sim state
		p.Guest = entities.NewGuest(image.Pt(at.X+(4-int(f))*world.TileSize*3/2, at.Y), witchName+"#"+strconv.Itoa(int(f)))
		p.Anim.Facing = f
		g.street.walkers = append(g.street.walkers, p)
	}
}
