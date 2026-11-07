package game

import (
	"cmp"
	"fmt"
	"image"
	"image/color"
	_ "image/png" // tile decoding
	"math"
	"slices"

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

// paint pre-draws a w×h-cell block at 1px per world px: layers [from, to) of at(layer, x, y).
func (g *Game) paint(w, h, from, to int, at func(l, x, y int) world.TileID) *ebiten.Image {
	img := ebiten.NewImage(w*world.TileSize, h*world.TileSize)
	for l := from; l < to; l++ {
		for y := range h {
			for x := range w {
				if id := at(l, x, y); id != 0 {
					op := &ebiten.DrawImageOptions{}
					op.GeoM.Translate(float64(x*world.TileSize), float64(y*world.TileSize))
					img.DrawImage(g.tiles[id], op)
				}
			}
		}
	}
	return img
}

// prop is one static tile above the floor (wall face, lintel, side line, window, counter), drawn in the
// y-sorted pass at its cell's SortY. Door leaves are left out: drawDoors slides them under the actors.
type prop struct {
	y, cx, cy int // sort y (world px), cell
	l         int // layer
	id        world.TileID
}

// sortedProps lists the props by sort y; within a cell, layers keep their order.
func (g *Game) sortedProps() []prop {
	m := g.hotel.Map
	var ps []prop
	for l := world.LayerFloor + 1; l < world.NumLayers; l++ {
		for y := range m.H {
			for x := range m.W {
				if id := m.At(l, x, y); id != 0 && !m.TS.Tiles[id].Door {
					ps = append(ps, prop{m.TileSortY(l, x, y), x, y, l, id})
				}
			}
		}
	}
	slices.SortStableFunc(ps, func(a, b prop) int { return cmp.Compare(a.y, b.y) })
	return ps
}

func rect(s *ebiten.Image, x, y, w, h int, c color.Color) {
	ebitenutil.DrawRect(s, float64(x), float64(y), float64(w), float64(h), c)
}
func text(s *ebiten.Image, v string, x, y int, _ color.Color) { ebitenutil.DebugPrintAt(s, v, x, y) }

func (g *Game) Draw(screen *ebiten.Image) {
	screen.Fill(assets.Night)
	if g.ghost && g.state != StateTitle {
		screen.Fill(color.Black)
	}
	if g.state == StateTitle {
		g.drawTitle(screen)
		return
	}
	mw, mh := g.mapSize()
	ox, oy := g.cam.Origin(g.player.X, g.player.Y, mw, mh)
	g.drawWorld(screen, ox, oy)
	// HUD and terminal: screen space
	if g.state == StatePlay && g.atTerminal() {
		text(screen, "[SPACE] OPEN TERMINAL", 28, 498, assets.Money)
	}
	if g.state == StatePlay && g.atMirror() {
		to := "GHOST"
		if g.ghost {
			to = "REAL"
		}
		text(screen, "[SPACE] SHIFT TO "+to, 28, 498, assets.Money)
	}
	if g.state == StateTerminal {
		g.drawTerminal(screen)
	}
	text(screen, g.message, 28, 518, assets.Accent)
}

func (g *Game) drawTitle(s *ebiten.Image) {
	text(s, "G H O S T   H O T E L", 345, 195, assets.Ghost)
	text(s, "[ SPACE ] open the hotel", 362, 335, assets.Money)
	text(s, "WASD move / SPACE use / ESC title or quit", 290, 485, assets.Floor)
}

// drawWorld draws the view with top-left (ox,oy) screen px ×zoom (nearest-neighbor) onto dst, at
// screen-px precision so sub-pixel motion scrolls smoothly.
func (g *Game) drawWorld(screen *ebiten.Image, ox, oy int) {
	if g.floorImg == nil { // only the floor is pre-drawn; everything above it is y-sorted with the actors (D11)
		m := g.hotel.Map
		g.floorImg, g.props = g.paint(m.W, m.H, world.LayerFloor, world.LayerFloor+1, m.At), g.sortedProps()
	}
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Scale(zoom, zoom)
	op.GeoM.Translate(float64(-ox), float64(-oy))
	g.blit(screen, g.floorImg, op, modeMaze, floorBlue, nil, 0)
	g.drawSpookFloor(screen, ox, oy)
	g.drawFogFloor(screen, ox, oy)
	g.drawDoors(screen, ox, oy)
	at := func(x, y float64) (int, int) { return screenPx(x) - ox, screenPx(y) - oy }
	type actor struct {
		x, y float64 // foot box center; y sorts (T039)
		name string
		anim entities.Anim
		fog  float64
		fear float64 // of fearLimit
	}
	actors := []actor{{g.player.X, g.player.Y, "Bob", g.player.Anim, 0, 0}}
	for _, guest := range g.guests() {
		actors = append(actors, actor{guest.X, guest.Y, guest.Character, guest.Anim, g.fogAt(image.Pt(int(guest.X), int(guest.Y))),
			min(1, float64(guest.Fear)/fearLimit)})
	}
	for _, pr := range g.priests {
		actors = append(actors, actor{pr.X, pr.Y, "Priest", pr.Anim, g.fogAt(image.Pt(int(pr.X), int(pr.Y))), 0})
	}
	slices.SortStableFunc(actors, func(a, b actor) int { return cmp.Compare(a.y, b.y) })
	// y-sorted pass (D11): merge the props and the actors; an actor whose feet are on or below a prop's
	// sort y is drawn over it
	i := 0
	propsUpTo := func(feet float64) {
		for ; i < len(g.props) && float64(g.props[i].y) <= feet; i++ {
			p := g.props[i]
			op.GeoM.Reset()
			op.GeoM.Scale(zoom, zoom)
			op.GeoM.Translate(float64((p.cx*world.TileSize+g.shake(p))*zoom-ox), float64(p.cy*world.TileSize*zoom-oy))
			s := g.spookAt(image.Pt(p.cx, p.cy))
			if f := g.propFog(p); f > 0 {
				drawFogged(screen, g.tiles[p.id], op, f)
			} else if maze(g.ts.Tiles[p.id].Name) {
				g.blit(screen, g.tiles[p.id], op, modeMaze, 1, nil, s)
			} else {
				g.blit(screen, g.tiles[p.id], op, modeProp, 0, nil, s)
			}
		}
	}
	for _, a := range actors {
		propsUpTo(a.y)
		x, y := at(a.x, a.y)
		if g.ghost && a.fear > 0 {
			g.drawFear(screen, x, y, a.fear)
		}
		g.drawCharacter(screen, x, y, a.name, a.anim, a.fog)
	}
	propsUpTo(math.Inf(1))
	if !g.ghost { // the palette replaces the day and night (D160)
		g.drawDarkness(screen, ox, oy)
	}
	for _, f := range g.floaters { // "+$X" rising over the door (D87); "+N" Spook in ghost only
		x, y := at(f.x, f.y-float64(g.tick-f.at)*floaterRisePerFr)
		switch {
		case !f.spook:
			text(screen, f.text, x-3*len(f.text), y, assets.Money)
		case g.ghost:
			g.cyan(screen, f.text, x-3*len(f.text), y)
		}
	}
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
				g.blit(screen, g.tiles[id].SubImage(src).(*ebiten.Image), op, modeProp, 0, nil, g.roomShift(d.Room))
			}
		}
	}
}

// Character frames are 16x32; strips hold 4 facings x 6 frames (D26).
const charW, charH = 16, 32

// loadCharacters decodes the idle_anim and run sheets of every character, by name.
func loadCharacters(names ...string) (map[string][2]*ebiten.Image, error) {
	m := map[string][2]*ebiten.Image{}
	for _, name := range names {
		var sheets [2]*ebiten.Image // idle, run
		for i, kind := range [...]string{"idle_anim", "run"} {
			img, _, err := ebitenutil.NewImageFromFileSystem(assets.FS, "characters/"+name+"_"+kind+"_16x16.png")
			if err != nil {
				return nil, fmt.Errorf("character %s: %w", name, err)
			}
			sheets[i] = img
		}
		m[name] = sheets
	}
	return m, nil
}

// drawCharacter draws the current frame with its bottom center on the foot box's bottom center (T039).
// (x,y) is the foot box center in screen px; inside a fogged Room it fades out with the fog (D92).
func (g *Game) drawCharacter(s *ebiten.Image, x, y int, name string, a entities.Anim, fog float64) {
	sheet := g.characters[name][0]
	if a.Moving {
		sheet = g.characters[name][1]
	}
	f := a.Frame()
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Scale(zoom, zoom)
	op.GeoM.Translate(float64(x-charW/2*zoom), float64(y+(entities.FootH/2-charH)*zoom))
	op.ColorScale.ScaleAlpha(float32(1 - fog))
	g.blit(s, sheet.SubImage(image.Rect(f*charW, 0, (f+1)*charW, charH)).(*ebiten.Image), op, modeActor, 0, ghostTints[name], 0)
}
