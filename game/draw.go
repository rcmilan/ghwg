package game

import (
	"cmp"
	"fmt"
	"image"
	"image/color"
	_ "image/png" // tile decoding
	"math"
	"slices"

	"shift/assets"
	"shift/entities"
	"shift/world"

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
	if g.state == StateTitle {
		g.drawTitle(screen)
		return
	}
	mw, mh := g.mapSize()
	ox, oy := g.cam.Origin(g.player.X, g.player.Y, mw, mh)
	if f := g.spinView(); f > 0 {
		g.drawSpinView(screen, ox, oy, f)
	} else {
		g.drawWorld(screen, ox, oy)
	}
	// HUD and terminal: screen space
	if g.state == StatePlay && !g.spinning() && g.atTerminal() {
		text(screen, "[SPACE] OPEN TERMINAL", 28, 498, assets.Money)
	}
	if room := g.atPanel(); g.state == StatePlay && !g.spinning() && room >= 0 {
		to := "AUTO"
		if g.autoRoom[room] {
			to = "MANUAL"
		}
		text(screen, "[SPACE] SET ROOM "+g.hotel.Rooms[room].Number+" "+to, 28, 498, assets.Money)
	}
	if g.state == StateTerminal {
		g.drawTerminal(screen)
	}
	text(screen, g.message, 28, 518, assets.Accent)
}

func (g *Game) drawTitle(s *ebiten.Image) {
	text(s, "S H I F T", 381, 195, assets.Pale)
	text(s, "[ SPACE ] open the hotel", 362, 335, assets.Money)
	text(s, "WASD move / SPACE terminal / ESC title or quit", 275, 485, assets.Floor)
}

// drawSpinView draws the Spin cinematic (D48): the whole map ×zoom off screen, scaled onto the screen
// between the camera's view (f=0, origin ox,oy) and the all-Rooms view (f=1). Zoom blends geometrically,
// the center linearly; linear filtering keeps the scaled pixel art from shimmering.
func (g *Game) drawSpinView(screen *ebiten.Image, ox, oy int, f float64) {
	mw, mh := g.mapSize()
	if g.worldImg == nil || g.worldImg.Bounds().Dx() != mw*zoom || g.worldImg.Bounds().Dy() != mh*zoom {
		g.worldImg = ebiten.NewImage(mw*zoom, mh*zoom)
	}
	g.worldImg.Fill(assets.Night)
	g.drawWorld(g.worldImg, 0, 0)
	const sw, sh = viewW * zoom, viewH * zoom
	cx0, cy0 := (float64(ox)+sw/2)/zoom, (float64(oy)+sh/2)/zoom
	cx1, cy1, z1 := g.roomsView()
	z := zoom * math.Pow(z1/zoom, f)
	cx, cy := cx0+(cx1-cx0)*f, cy0+(cy1-cy0)*f
	op := &ebiten.DrawImageOptions{Filter: ebiten.FilterLinear}
	op.GeoM.Scale(z/zoom, z/zoom)
	op.GeoM.Translate(sw/2-cx*z, sh/2-cy*z)
	screen.DrawImage(g.worldImg, op)
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
	screen.DrawImage(g.floorImg, op)
	g.drawFogFloor(screen, ox, oy)
	reels := g.drawReels(screen, ox, oy)
	still := func(p prop) bool { // a rolling Room's tiles roll, except the hotel's outer side line over them
		for _, r := range reels {
			if image.Pt(p.cx, p.cy).In(r) {
				return p.l == world.LayerDecor && g.hotel.OuterLine(p.cx)
			}
		}
		return true
	}
	g.drawDoors(screen, ox, oy, reels)
	at := func(x, y float64) (int, int) { return screenPx(x) - ox, screenPx(y) - oy }
	type drawable struct {
		y    float64 // world px; sorts with the props (T039): an actor's feet, a panel's wall row
		draw func()
	}
	character := func(x, y float64, name string, a entities.Anim, fog float64) drawable {
		return drawable{y, func() { sx, sy := at(x, y); g.drawCharacter(screen, sx, sy, name, a, fog) }}
	}
	ds := []drawable{character(g.player.X, g.player.Y, "Bob", g.player.Anim, 0)}
	for _, guest := range g.guests() {
		ds = append(ds, character(guest.X, guest.Y, guest.Character, guest.Anim, g.fogAt(image.Pt(int(guest.X), int(guest.Y)))))
	}
	for i, r := range g.hotel.Rooms {
		p := r.Panel()
		ds = append(ds, drawable{float64(g.hotel.Map.SortY(p.X, p.Y)), func() { g.drawPanel(screen, p, g.autoRoom[i], ox, oy) }})
	}
	slices.SortStableFunc(ds, func(a, b drawable) int { return cmp.Compare(a.y, b.y) })
	// y-sorted pass (D11): merge the props and the drawables; one whose y is on or below a prop's sort y is
	// drawn over it
	i := 0
	propsUpTo := func(feet float64) {
		for ; i < len(g.props) && float64(g.props[i].y) <= feet; i++ {
			p := g.props[i]
			if !still(p) {
				continue
			}
			op.GeoM.Reset()
			op.GeoM.Scale(zoom, zoom)
			op.GeoM.Translate(float64(p.cx*world.TileSize*zoom-ox), float64(p.cy*world.TileSize*zoom-oy))
			drawFogged(screen, g.tiles[p.id], op, g.propFog(p))
		}
	}
	for _, d := range ds {
		propsUpTo(d.y)
		d.draw()
	}
	propsUpTo(math.Inf(1))
	g.drawDarkness(screen, ox, oy)
	for _, f := range g.floaters { // "+$X" rising over the door (D87)
		x, y := at(f.x, f.y-float64(g.tick-f.at)*floaterRisePerFr)
		text(screen, f.text, x-3*len(f.text), y, assets.Money)
	}
}

// Door Panel, in world px within its wall cell (04 D117): a dark plate with a light near its top.
const panelX, panelY, panelW, panelH, panelLight = 5, 3, 6, 9, 2

// drawPanel draws a Room's Door Panel on its wall cell: green while the Room is auto, amber while manual.
func (g *Game) drawPanel(s *ebiten.Image, cell image.Point, auto bool, ox, oy int) {
	x, y := (cell.X*world.TileSize+panelX)*zoom-ox, (cell.Y*world.TileSize+panelY)*zoom-oy
	rect(s, x, y, panelW*zoom, panelH*zoom, assets.Ink)
	rect(s, x+(panelW-panelLight)/2*zoom, y+2*zoom, panelLight*zoom, panelLight*zoom, panelColor(auto))
}

func panelColor(auto bool) color.RGBA {
	if auto {
		return assets.Green
	}
	return assets.Amber
}

// drawReels draws each rolling Room's whole footprint, clipped to it (D48): the old cells slide out the
// bottom while the new type comes in from the top. Actors, props outside the reel and the hotel's outer
// side line still draw over it. Returns each Room's rolling cells, empty when still.
func (g *Game) drawReels(screen *ebiten.Image, ox, oy int) []image.Rectangle {
	const ts, rh = world.TileSize, world.RoomH * world.TileSize * zoom
	m := g.hotel.Map
	rects := make([]image.Rectangle, len(g.hotel.Rooms))
	if g.reels == nil {
		g.reels = make([][2]*ebiten.Image, len(g.hotel.Rooms))
	}
	for i, r := range g.hotel.Rooms {
		f, rolling := g.reel(i)
		if !rolling {
			continue
		}
		o, t := r.Origin, r.Type
		if g.reels[i][0] == nil { // the map still holds the old cells until this Room lands
			line := func(l, x int) bool { return l == world.LayerDecor && g.hotel.OuterLine(o.X+x) } // drawn still, on top
			g.reels[i] = [2]*ebiten.Image{
				g.paint(world.RoomW, world.RoomH, 0, world.NumLayers, func(l, x, y int) world.TileID {
					if line(l, x) {
						return 0
					}
					return m.At(l, o.X+x, o.Y+y)
				}),
				g.paint(world.RoomW, world.RoomH, 0, world.NumLayers, func(l, x, y int) world.TileID {
					if line(l, x) {
						return 0
					}
					return t.Tiles[l][y*world.RoomW+x]
				}),
			}
		}
		rects[i] = image.Rect(o.X, o.Y, o.X+world.RoomW, o.Y+world.RoomH)
		px := image.Rectangle{rects[i].Min.Mul(ts * zoom), rects[i].Max.Mul(ts * zoom)}.Sub(image.Pt(ox, oy))
		clip := screen.SubImage(px).(*ebiten.Image)
		clip.Fill(assets.Night)
		dy := math.Round(f * rh)
		for k, img := range g.reels[i] {
			op := &ebiten.DrawImageOptions{}
			op.GeoM.Scale(zoom, zoom)
			op.GeoM.Translate(float64(o.X*ts*zoom-ox), float64(o.Y*ts*zoom-oy)+dy-float64(k*rh))
			clip.DrawImage(img, op)
		}
	}
	return rects
}

// drawDoors draws each door's leaves slid by its Open amount, clipped to their own cells: a Room door
// slides right; the Entrance's leaves slide apart. A rolling Room's door rolls with its reel instead.
func (g *Game) drawDoors(screen *ebiten.Image, ox, oy int, reels []image.Rectangle) {
	const ts = world.TileSize
	m := g.hotel.Map
	for _, d := range g.hotel.Doors {
		if d.Room >= 0 && !reels[d.Room].Empty() {
			continue
		}
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
	s.DrawImage(sheet.SubImage(image.Rect(f*charW, 0, (f+1)*charW, charH)).(*ebiten.Image), op)
}
