package game

import (
	"fmt"
	"image/color"
	_ "image/png" // tile decoding
	"time"

	"ghosthotel/assets"
	"ghosthotel/entities"
	"ghosthotel/world"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
)

// loadTiles decodes every tileset image once, indexed by world.TileID.
func loadTiles(ts *world.Tileset) ([]*ebiten.Image, error) {
	imgs := make([]*ebiten.Image, len(ts.Tiles))
	for id, t := range ts.Tiles {
		if t.Image == "" {
			continue
		}
		img, _, err := ebitenutil.NewImageFromFileSystem(assets.FS, t.Image)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", t.Image, err)
		}
		imgs[id] = img
	}
	return imgs, nil
}

// renderMap pre-draws the static map: floor, walls, decor. Door leaves are skipped
// (ponytail: doors drawn open until S4 animates them).
func (g *Game) renderMap() *ebiten.Image {
	m := g.hotel.Map
	img := ebiten.NewImage(m.W*world.TileSize, m.H*world.TileSize)
	for l := range world.NumLayers {
		for y := range m.H {
			for x := range m.W {
				id := m.At(l, x, y)
				if id == 0 || m.TS.Tiles[id].Door {
					continue
				}
				op := &ebiten.DrawImageOptions{}
				op.GeoM.Translate(float64(x*world.TileSize), float64(y*world.TileSize))
				img.DrawImage(g.tiles[id], op)
			}
		}
	}
	return img
}

func rect(s *ebiten.Image, x, y, w, h int, c color.Color) {
	ebitenutil.DrawRect(s, float64(x), float64(y), float64(w), float64(h), c)
}
func text(s *ebiten.Image, v string, x, y int, _ color.Color) { ebitenutil.DebugPrintAt(s, v, x, y) }

func (g *Game) Draw(screen *ebiten.Image) {
	screen.Fill(assets.Night)
	if g.state == StateTitle {
		g.drawTitle(screen)
		return
	}
	g.drawWorld(screen)
	// HUD and terminal: screen space
	if g.state == StateBooking && g.atTerminal() {
		text(screen, "[SPACE] OPEN TERMINAL", 28, 498, assets.Money)
	}
	if g.state == StateDay {
		left := dayDuration - time.Since(g.dayStarted)
		text(screen, fmt.Sprintf("DAY %02ds", int(left.Seconds())+1), 440, 12, assets.Money)
	}
	if g.state == StateTerminal {
		g.drawTerminal(screen)
	}
	text(screen, g.message, 28, 518, assets.Accent)
	text(screen, fmt.Sprintf("$%02d", g.money), 880, 518, assets.Money)
}

func (g *Game) drawTitle(s *ebiten.Image) {
	text(s, "G H O S T   H O T E L", 345, 195, assets.Ghost)
	text(s, "[ SPACE ] begin shift", 370, 335, assets.Money)
	text(s, "WASD move / SPACE interact / ESC title or quit", 275, 485, assets.Floor)
}

// drawWorld draws the camera's view ×zoom (nearest-neighbor) straight onto the screen, at screen-px
// precision so sub-pixel motion scrolls smoothly.
func (g *Game) drawWorld(screen *ebiten.Image) {
	if g.mapImg == nil {
		g.mapImg = g.renderMap()
	}
	mw, mh := g.mapSize()
	ox, oy := g.cam.Origin(g.player.X, g.player.Y, mw, mh)
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Scale(zoom, zoom)
	op.GeoM.Translate(float64(-ox), float64(-oy))
	screen.DrawImage(g.mapImg, op)
	at := func(x, y float64) (int, int) { return screenPx(x) - ox, screenPx(y) - oy }
	// ponytail: actors draw above all tiles until S7's y-sort
	for _, coin := range g.pickups {
		x, y := at(coin.X, coin.Y)
		rect(screen, x-2*zoom, y-2*zoom, 4*zoom, 3*zoom, assets.Money)
	}
	for _, guest := range g.guests {
		if guest.State != entities.GuestQueued {
			x, y := at(guest.X, guest.Y)
			drawPerson(screen, x, y, assets.Guest)
		}
	}
	x, y := at(g.player.X, g.player.Y)
	drawPerson(screen, x, y, assets.Ghost)
}

// drawPerson: placeholder body standing on its foot box (feet at y+FootH/2), in screen px, until S6 sprites.
func drawPerson(s *ebiten.Image, x, y int, c color.Color) {
	feet := y + entities.FootH/2*zoom
	rect(s, x-4*zoom, feet-14*zoom, 8*zoom, 14*zoom, c)
	rect(s, x-3*zoom, feet-19*zoom, 6*zoom, 5*zoom, c)
}

func (g *Game) drawTerminal(s *ebiten.Image) {
	rect(s, 220, 85, 520, 340, assets.Ink)
	rect(s, 228, 93, 504, 324, assets.Wall)
	text(s, "RECEPTION TERMINAL", 365, 125, assets.Ghost)
	text(s, "W/S SELECT   A/D UNLOCKED ROOM   SPACE LOCK/UNLOCK", 255, 155, assets.Accent)
	for i, room := range g.assignments {
		mark, status := " ", "UNBOOKED"
		if i == g.terminalCursor {
			mark = ">"
		}
		if g.confirmed[i] {
			status = "BOOKED"
		}
		text(s, fmt.Sprintf("%s GUEST %d -> %s  [%s]", mark, i+1, g.hotel.Rooms[room].Number, status), 280, 205+i*42, assets.Money)
	}
	mark := " "
	if g.terminalCursor == len(g.assignments) {
		mark = ">"
	}
	text(s, mark+" BEGIN ARRIVALS", 280, 375, assets.Ghost)
}
