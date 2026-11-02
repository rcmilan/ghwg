package game

import (
	"fmt"
	"image"
	"image/color"
	_ "image/png" // tile decoding
	"math"

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

// renderMap pre-draws the map tiles for which keep is true, all layers in order.
func (g *Game) renderMap(keep func(world.Tile) bool) *ebiten.Image {
	m := g.hotel.Map
	img := ebiten.NewImage(m.W*world.TileSize, m.H*world.TileSize)
	for l := range world.NumLayers {
		for y := range m.H {
			for x := range m.W {
				id := m.At(l, x, y)
				if id == 0 || !keep(m.TS.Tiles[id]) {
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

// isLintel: the wall top over a doorway, archway or the Entrance. Actors pass under it.
func isLintel(t world.Tile) bool { return t.Name == "wall_lintel" }

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
		left := dayFrames - (g.tick - g.dayStarted)
		text(screen, fmt.Sprintf("DAY %02ds", (left+tps-1)/tps), 440, 12, assets.Money)
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
		// door leaves are skipped (drawDoors slides them), lintels go over the actors
		g.mapImg = g.renderMap(func(t world.Tile) bool { return !t.Door && !isLintel(t) })
		g.overImg = g.renderMap(isLintel)
	}
	mw, mh := g.mapSize()
	ox, oy := g.cam.Origin(g.player.X, g.player.Y, mw, mh)
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Scale(zoom, zoom)
	op.GeoM.Translate(float64(-ox), float64(-oy))
	screen.DrawImage(g.mapImg, op)
	g.drawDoors(screen, ox, oy)
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
	screen.DrawImage(g.overImg, op) // ponytail: only lintels go over actors; S7's y-sort generalizes depth
}

// drawDoors draws each door's leaves slid by its Open amount, clipped to their own cells: a Room door
// slides right; the Entrance's leaves slide apart.
func (g *Game) drawDoors(screen *ebiten.Image, ox, oy int) {
	const ts = world.TileSize
	m := g.hotel.Map
	for _, d := range g.hotel.Doors {
		off := int(math.Round(d.Open() * ts))
		for y := d.Rect.Min.Y / ts; y < d.Rect.Max.Y/ts; y++ {
			for x := d.Rect.Min.X / ts; x < d.Rect.Max.X/ts; x++ {
				id := m.At(world.LayerDecor, x, y)
				if !m.TS.Tiles[id].Door || off == ts {
					continue
				}
				src, dx := image.Rect(0, 0, ts-off, ts), off // slides right: its left part shows, shifted
				if d.Room < 0 && x == d.Rect.Min.X/ts {
					src, dx = image.Rect(off, 0, ts, ts), 0 // Entrance's left leaf slides left
				}
				op := &ebiten.DrawImageOptions{}
				op.GeoM.Scale(zoom, zoom)
				op.GeoM.Translate(float64((x*ts+dx)*zoom-ox), float64(y*ts*zoom-oy))
				screen.DrawImage(g.tiles[id].SubImage(src).(*ebiten.Image), op)
			}
		}
	}
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
