package game

import (
	"cmp"
	"fmt"
	"image"
	"image/color"
	_ "image/png" // tile decoding
	"math"
	"math/rand/v2"
	"slices"
	"strconv"
	"strings"

	"ghosthotel/assets"
	"ghosthotel/entities"
	"ghosthotel/world"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/colorm"
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
func text(s *ebiten.Image, v string, x, y int, _ color.Color) { printAt(s, v, x, y, 1, color.White, 1) }

// inkBuf: the debug font's glyphs carry a baked half-alpha black shadow; printed here, then drawn with
// alpha := red, only the white glyph pixels stay (no text shadows, 2026-11-08).
var inkBuf = ebiten.NewImage(960, 64)

// printAt prints v in the debug font at (x,y), at scale, in c at alpha a. Every text goes through it.
func printAt(s *ebiten.Image, v string, x, y int, scale float64, c color.Color, a float32) {
	lines := strings.Split(v, "\n")
	w := 0
	for _, l := range lines {
		w = max(w, len(l))
	}
	buf := inkBuf.SubImage(image.Rect(0, 0, min(w*glyphW+1, 960), min(len(lines)*glyphH, 64))).(*ebiten.Image)
	buf.Clear()
	ebitenutil.DebugPrint(buf, v)
	var cm colorm.ColorM
	cm.SetElement(3, 0, 1) // alpha := red: white glyph 1, black shadow 0
	cm.SetElement(3, 3, 0)
	cm.ScaleWithColor(c)
	cm.Scale(1, 1, 1, float64(a))
	op := &colorm.DrawImageOptions{}
	op.GeoM.Scale(scale, scale)
	op.GeoM.Translate(float64(x), float64(y))
	colorm.DrawImage(s, buf, cm, op)
}

func (g *Game) Draw(screen *ebiten.Image) {
	if g.shifting != nil && g.state != StateTitle {
		g.drawShift(screen)
		return
	}
	if !g.warm && g.state != StateTitle && g.fx[0] != nil {
		g.warmUp(screen)
	}
	g.drawFrame(screen)
}

// drawFrame draws the screen in the current dimension.
func (g *Game) drawFrame(screen *ebiten.Image) {
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
	// HUD and terminal: screen space. No "[SPACE] ..." prompts: props must read on their own (2026-11-08).
	if g.state == StateTerminal {
		g.drawTerminal(screen)
	}
	g.drawHUD(screen) // over the Terminal, which has no wallet label of its own
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
	g.drawMoodFloor(screen, ox, oy)
	g.drawFogFloor(screen, ox, oy)
	g.drawDoors(screen, ox, oy)
	g.drawSupplies(screen, ox, oy)
	at := func(x, y float64) (int, int) { return screenPx(x) - ox, screenPx(y) - oy }
	type actor struct {
		x, y float64 // foot box center; y sorts (T039)
		name string
		anim entities.Anim
		fog  float64
		fear float64   // of its FleeAt
		lift float64   // world px off the floor: a fleeing Guest's hop (D164)
		tint []float32 // ghost dimension: its flat color; nil for the character's own
		a    float32   // opacity; 0 is 1
		mode int       // the palette shader's mode; 0 is modeActor
		sil  bool      // a human seen from the ghost dimension: a white silhouette with a small glow
	}
	actors := []actor{{x: g.player.X, y: g.player.Y, name: "Bob", anim: g.player.Anim}}
	for _, realm := range [...]Realm{Human, Ghost} {
		if realm == Ghost && !g.ghost { // in real a ghost shows only through its area's Shadows
			continue
		}
		for _, guest := range g.guestsIn(realm) {
			a := actor{x: guest.X, y: guest.Y, name: guest.Character, anim: guest.Anim, fog: g.fogAt(image.Pt(int(guest.X), int(guest.Y))),
				fear: min(1, float64(guest.Fear)/float64(guest.FleeAt)), lift: g.lift(guest)}
			switch {
			case realm == Ghost: // hued by its kind
				h := kindHue(kindOf(guest.Phantasm))
				a.tint, a.a, a.mode = h[:], ghostAlpha, modeFlat
			case g.ghost: // a human in the ghost dimension: a silhouette
				a.tint, a.a, a.mode, a.sil = silhouetteTint, silhouetteAlpha, modeFlat, true
			}
			actors = append(actors, a)
		}
	}
	for _, pr := range g.priests {
		actors = append(actors, actor{x: pr.X, y: pr.Y, name: pr.Character, anim: pr.Anim, fog: g.fogAt(image.Pt(int(pr.X), int(pr.Y)))})
	}
	for _, c := range g.couriers {
		name := courierName
		if len(c.drops) > 0 {
			name = courierBox
		}
		actors = append(actors, actor{x: c.X, y: c.Y, name: name, anim: c.Anim, fog: g.fogAt(image.Pt(int(c.X), int(c.Y)))})
	}
	z := func(a actor) float64 { // sort y: in bed, the bed's bottom, so the blanket goes over it (D167)
		if b, ok := g.bedAt(a.x, a.y); ok {
			return float64(b.Max.Y)
		}
		return a.y
	}
	slices.SortStableFunc(actors, func(a, b actor) int { return cmp.Compare(z(a), z(b)) })
	// y-sorted pass (D11): merge the props and the actors; an actor whose feet are on or below a prop's
	// sort y is drawn over it
	drawProp := func(p prop) {
		op.GeoM.Reset()
		op.GeoM.Scale(zoom, zoom)
		op.GeoM.Translate(float64(p.cx*world.TileSize*zoom-ox), float64(p.cy*world.TileSize*zoom-oy))
		s := g.dreadAt(image.Pt(p.cx, p.cy))
		img := g.tiles[p.id]
		if g.covered(p) { // the lobby Mirror, until it's installed
			img = g.coverHalf(g.ts.Tiles[p.id].Name)
		}
		if f := g.propFog(p); f > 0 {
			drawFogged(screen, img, op, f)
		} else if maze(g.ts.Tiles[p.id].Name) {
			g.blit(screen, img, op, modeMaze, 1, nil, s)
		} else {
			g.blit(screen, img, op, modeProp, 0, nil, s)
		}
		if p.l == world.LayerWalls {
			g.drawShadow(screen, p, ox, oy)
		}
	}
	g.bucketShadows()
	i := 0
	propsUpTo := func(y float64) {
		for ; i < len(g.props) && float64(g.props[i].y) <= y; i++ {
			drawProp(g.props[i])
		}
	}
	propsUpTo(0) // the rugs, under the stains; furniture goes over them (Live test 7, 2026-11-08)
	g.drawDirt(screen, ox, oy)
	for _, a := range actors {
		propsUpTo(z(a))
		x, y := at(a.x, a.y-a.lift)
		if g.ghost && a.fear >= fearWarn { // near fleeing: it trembles (2026-11-08)
			x += []int{-1, 0, 1, 0}[g.tick/3%4] * zoom
		}
		if a.sil {
			g.drawGlow(screen, x, y, [3]float32{1, 1, 1}, silhouetteGlow, silhouetteGlowScale)
		}
		if g.ghost && a.fear > 0 {
			g.drawFear(screen, x, y, a.fear)
		}
		g.drawCharacterAs(screen, x, y, a.name, a.anim, a.fog, a.tint, a.a, a.mode)
		if b, ok := g.bedAt(a.x, a.y); ok { // the blanket: the bed's bottom row, from under the pillow, again over the sleeper
			c := image.Pt(b.Min.X, b.Max.Y-1).Div(world.TileSize)
			for _, p := range g.props {
				if p.cx == c.X && p.cy == c.Y {
					drawProp(p)
				}
			}
		}
	}
	propsUpTo(math.Inf(1))
	if hx, hy := at(g.player.X, g.player.Y); g.hands != "" {
		g.drawHands(screen, hx, hy)
	}
	if !g.ghost { // the palette replaces the day and night (D160)
		g.drawDarkness(screen, ox, oy)
	}
	g.drawRequests(screen, ox, oy)
	g.drawTalk(screen, ox, oy)
	for _, f := range g.floaters { // "+$X" rising over the door (D87); "+N" Ecto in ghost only
		x, y := at(f.x, f.y-float64(g.tick-f.at)*floaterRisePerFr)
		switch {
		case f.kind == floatMoney || f.kind == floatEcto && g.ghost: // a currency's kind is its Res (wallet.go)
			g.tinted(screen, f.text, x-3*len(f.text), y, resInk(Res(f.kind)), 1)
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

// loadCharacters decodes the idle_anim and run sheets of every character, by name, as pixels to recolor
// (D166).
func loadCharacters(names ...string) (map[string][2]*image.RGBA, error) {
	m := map[string][2]*image.RGBA{}
	for _, name := range names {
		var sheets [2]*image.RGBA // idle, run
		for i, kind := range [...]string{"idle_anim", "run"} {
			f, err := assets.FS.Open("characters/" + name + "_" + kind + "_16x16.png")
			if err != nil {
				return nil, fmt.Errorf("character %s: %w", name, err)
			}
			img, _, err := image.Decode(f)
			f.Close()
			if err != nil {
				return nil, fmt.Errorf("character %s: %w", name, err)
			}
			sheets[i] = image.NewRGBA(img.Bounds())
			for y := range img.Bounds().Dy() {
				for x := range img.Bounds().Dx() {
					sheets[i].Set(x, y, img.At(x, y))
				}
			}
		}
		m[name] = sheets
	}
	return m, nil
}

// dress: a character's sheets recolored to l, as images.
func dress(sheets [2]*image.RGBA, pal map[uint32][2]region, l look) [2]*ebiten.Image {
	return [2]*ebiten.Image{ebiten.NewImageFromImage(recolor(sheets[0], pal, l)), ebiten.NewImageFromImage(recolor(sheets[1], pal, l))}
}

// sheet: name's idle and run sheets. A Guest's name is its base character, "#" and its look's seed
// (D166); its sheets are recolored on first use.
// ponytail: a Guest's sheets are never freed; a Run makes a few dozen 384x32 pairs.
func (g *Game) sheet(name string) [2]*ebiten.Image {
	s, ok := g.characters[name]
	if !ok {
		base, key, _ := strings.Cut(name, "#")
		seed, _ := strconv.ParseUint(key, 10, 64)
		s = dress(g.bases[base], palettes[base], randomLook(rand.New(rand.NewPCG(seed, 0))))
		g.characters[name] = s
	}
	return s
}

// drawCharacter draws the current frame with its bottom center on the foot box's bottom center (T039).
// (x,y) is the foot box center in screen px; inside a fogged Room it fades out with the fog (D92).
func (g *Game) drawCharacter(s *ebiten.Image, x, y int, name string, a entities.Anim, fog float64) {
	g.drawCharacterAs(s, x, y, name, a, fog, nil, 0, modeActor)
}

// drawCharacterAs is drawCharacter with the ghost dimension's color given (nil: the character's own) and
// an opacity (0: opaque).
func (g *Game) drawCharacterAs(s *ebiten.Image, x, y int, name string, a entities.Anim, fog float64, tint []float32, alpha float32, mode int) {
	if tint == nil {
		tint = ghostTints[strings.Split(name, "#")[0]]
	}
	if alpha == 0 {
		alpha = 1
	}
	sheet := g.sheet(name)[0]
	if a.Moving {
		sheet = g.sheet(name)[1]
	}
	f := a.Frame()
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Scale(zoom, zoom)
	op.GeoM.Translate(float64(x-charW/2*zoom), float64(y+(entities.FootH/2-charH)*zoom))
	op.ColorScale.ScaleAlpha(float32(1-fog) * alpha)
	g.blit(s, sheet.SubImage(image.Rect(f*charW, 0, (f+1)*charW, charH)).(*ebiten.Image), op, mode, 0, tint, 0)
}
