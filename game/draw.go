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

// paint pre-draws a w×h-cell block at 1px per world px: layers [from, to) of at(layer, x, y), each cell at
// origin(x, y).
func (g *Game) paint(w, h, from, to int, at func(l, x, y int) world.TileID, origin func(x, y int) image.Point) *ebiten.Image {
	img := ebiten.NewImage(w*world.TileSize, h*world.TileSize)
	for l := from; l < to; l++ {
		for y := range h {
			for x := range w {
				if id := at(l, x, y); id != 0 {
					op := &ebiten.DrawImageOptions{}
					op.GeoM.Translate(float64(origin(x, y).X), float64(origin(x, y).Y))
					img.DrawImage(g.tiles[id], op)
				}
			}
		}
	}
	return img
}

// onGrid: a cell's px in a block of its own, as paint's origin.
func onGrid(x, y int) image.Point { return image.Pt(x, y).Mul(world.TileSize) }

// paintFloor pre-draws the map's floor. H1: the hall's rows stand Slip px east of the grid, so their first
// tile is drawn once more to its west: the street runs to the map's edge.
func (g *Game) paintFloor() *ebiten.Image {
	m := g.hotel.Map
	img := g.paint(m.W, m.H, world.LayerFloor, world.LayerFloor+1, m.At, m.Origin)
	for y := m.SlipRow; m.Slip > 0 && y < m.H; y++ {
		op := &ebiten.DrawImageOptions{}
		op.GeoM.Translate(float64(m.Slip-world.TileSize), float64(y*world.TileSize))
		img.DrawImage(g.tiles[m.At(world.LayerFloor, 0, y)], op)
	}
	return img
}

// prop is one static tile above the floor (wall face, lintel, side line, window, counter), drawn in the
// y-sorted pass at its cell's SortY. Door leaves are left out: drawDoors slides them under the actors.
type prop struct {
	y, cx, cy int // sort y (world px), cell
	l         int // layer
	id        world.TileID
	band      *ebiten.Image // its part of the walls' top band, drawn over it (world/band.go); nil without one
	bx, by    int           // where the band's part starts, in cell px
}

// sortedProps lists the props by sort y; within a cell, layers keep their order.
func (g *Game) sortedProps() []prop {
	m := g.hotel.Map
	bands := m.Bands()
	var ps []prop
	for l := world.LayerFloor + 1; l < world.NumLayers; l++ {
		for y := range m.H {
			for x := range m.W {
				if id := m.At(l, x, y); id != 0 && !m.TS.Tiles[id].Door {
					p := prop{y: m.TileSortY(l, x, y), cx: x, cy: y, l: l, id: id}
					if r := world.BandRect(m.TS.Tiles[id].Name); !r.Empty() {
						p.band, p.bx, p.by = ebiten.NewImageFromImage(bands.SubImage(r.Add(m.Origin(x, y)))), r.Min.X, r.Min.Y
					}
					ps = append(ps, p)
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
	if g.intro != nil { // the opening draws its own, title included (intro.go)
		g.drawIntro(screen)
		return
	}
	var bg color.Color = assets.Night
	if g.ghost && g.state != StateTitle {
		bg = color.Black
	}
	screen.Fill(bg)
	if g.state == StateTitle {
		g.drawTitle(screen)
		return
	}
	mw, mh := g.mapSize()
	ox, oy := g.cam.Origin(g.player.X, g.player.Y, mw, mh)
	if g.state == StateTalk {
		g.drawParley(screen, bg)
	} else if g.viewT > 0 || g.state == StateGrow {
		g.drawView(screen, bg)
	} else {
		g.drawWorld(screen, ox, oy)
	}
	// HUD and terminal: screen space. No "[SPACE] ..." prompts: props must read on their own (2026-11-08).
	if g.state == StateTerminal {
		g.drawTerminal(screen)
	}
	if g.state == StatePause {
		g.drawPause(screen)
	}
	g.drawHUD(screen) // over the Terminal, which has no wallet label of its own; the notice too: there is no message line
}

func (g *Game) drawTitle(s *ebiten.Image) {
	for _, l := range [...]struct {
		v    string
		x, y int
	}{{"G H O S T   H O T E L", 345, 195}, {"[ SPACE ] open the hotel", 362, 335}, {"WASD move / SPACE use, or the hotel / ESC pause", 284, 485}} {
		printAt(s, l.v, l.x, l.y, 1, color.White, 1)
	}
}

// drawWorld draws the view with top-left (ox,oy) screen px ×zoom (nearest-neighbor) onto dst, at
// screen-px precision so sub-pixel motion scrolls smoothly.
func (g *Game) drawWorld(screen *ebiten.Image, ox, oy int) {
	if g.floorImg == nil { // only the floor is pre-drawn; everything above it is y-sorted with the actors (D11)
		g.floorImg, g.props = g.paintFloor(), g.sortedProps()
	}
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Scale(zoom, zoom)
	op.GeoM.Translate(float64(-ox), float64(-oy))
	g.blit(screen, g.floorImg, op, modeMaze, floorBlue, nil, 0)
	g.drawRoomGlow(screen, op)
	g.drawMoodFloor(screen, ox, oy)
	g.drawFogFloor(screen, ox, oy)
	g.drawDoors(screen, ox, oy)
	g.drawReels(screen, ox, oy)
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
		up   bool      // a ghost, or the Concierge in ghost: it floats and casts no shadow (F0)
		pot  bool      // a plant (F0 Decor): name is its sprite, drawn standing on (x,y)
	}
	var actors []actor
	if g.intro == nil || g.intro.phase == introEnter { // the opening hides the Concierge until he walks in
		actors = append(actors, actor{x: g.player.X, y: g.player.Y, name: "Bob", anim: g.player.Anim, up: g.ghost})
	}
	if g.intro != nil && g.ghost { // and has ghosts strolling the street
		for _, gh := range g.intro.ghosts {
			h := kindHue(kindOf(gh.Phantasm))
			actors = append(actors, actor{x: gh.X, y: gh.Y, name: gh.Character, anim: gh.Anim, tint: h[:], a: ghostAlpha, mode: modeFlat, up: true})
		}
	}
	for _, realm := range [...]Realm{Human, Ghost} {
		if realm == Ghost && !g.ghost { // in real a ghost shows only through its area's Shadows
			continue
		}
		for _, guest := range g.guestsIn(realm) {
			a := actor{x: guest.X, y: guest.Y, name: guest.Character, anim: guest.Anim, fog: g.fogAt(image.Pt(int(guest.X), int(guest.Y))),
				fear: min(1, float64(guest.Fear)/float64(guest.FleeAt)), lift: g.lift(guest)}
			switch {
			case realm == Ghost: // hued by its kind
				var up float64
				a.tint, a.a, up = g.ghostLook(guest)
				a.lift, a.mode, a.up = a.lift+up, modeFlat, true
			case g.ghost: // a human in the ghost dimension: a silhouette
				a.tint, a.a, a.mode, a.sil = silhouetteTint, silhouetteAlpha, modeFlat, true
			}
			actors = append(actors, a)
		}
	}
	for _, pr := range g.priests {
		actors = append(actors, actor{x: pr.X, y: pr.Y, name: pr.Character, anim: pr.Anim, fog: g.fogAt(image.Pt(int(pr.X), int(pr.Y))), lift: pr.lift()})
	}
	for _, c := range g.couriers {
		if !c.out { // ordered, the store or the hotel view still open: he walks in once it closes
			continue
		}
		name := courierName
		if len(c.drops) > 0 {
			name = courierBox
		}
		actors = append(actors, actor{x: c.X, y: c.Y, name: name, anim: c.Anim, fog: g.fogAt(image.Pt(int(c.X), int(c.Y)))})
	}
	for _, k := range g.keepers {
		actors = append(actors, actor{x: k.X, y: k.Y, name: keeperName, anim: k.Anim, fog: g.fogAt(image.Pt(int(k.X), int(k.Y)))})
	}
	for _, p := range g.hotel.Plants {
		actors = append(actors, actor{x: float64(p.At.X), y: float64(p.At.Y), name: p.Sprite, pot: true})
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
		if g.reelingAt(p.l, p.cx, p.cy) { // rolling with its Room's reel
			return
		}
		op.GeoM.Reset()
		op.GeoM.Scale(zoom, zoom)
		o := g.hotel.Map.Origin(p.cx, p.cy)
		op.GeoM.Translate(float64(o.X*zoom-ox), float64(o.Y*zoom-oy))
		s := g.dreadAt(image.Pt(p.cx, p.cy))
		img := g.tiles[p.id]
		if g.covered(p) { // the lobby Mirror, until it's installed
			img = g.coverHalf(g.ts.Tiles[p.id].Name)
		}
		put := func(img *ebiten.Image) {
			if f := g.propFog(p); f > 0 {
				drawFogged(screen, img, op, f)
			} else if maze(g.ts.Tiles[p.id].Name) {
				g.blit(screen, img, op, modeMaze, 1, nil, s)
			} else {
				g.blit(screen, img, op, modeProp, 0, nil, s)
			}
		}
		put(img)
		if p.band != nil {
			op.GeoM.Translate(float64(p.bx*zoom), float64(p.by*zoom))
			put(p.band)
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
	stockY, stocked := float64(g.hotel.Stock.Max.Y), false
	stock := func() { // the stock props: over the wall they stand against, under whoever stands in front
		if !stocked {
			propsUpTo(stockY)
			g.drawKitchen(screen, ox, oy)
			stocked = true
		}
	}
	_, lit := g.actionable() // the action glow (F0 Glow): over the prop it lights, under any actor standing in front of it
	glowed := lit.Empty() || g.state != StatePlay || g.intro != nil
	glow := func() {
		if !glowed {
			if lit.Max.Y <= int(stockY)+world.TileSize { // a kitchen shelf: its props first
				stock()
			}
			propsUpTo(float64(lit.Max.Y))
			g.drawActGlow(screen, ox, oy)
			glowed = true
		}
	}
	for _, a := range actors {
		if z(a) >= stockY {
			stock()
		}
		propsUpTo(z(a))
		if z(a) > float64(lit.Max.Y) {
			glow()
		}
		fx, fy := at(a.x, a.y)
		if a.pot {
			g.drawPlant(screen, a.name, fx, fy)
			continue
		}
		if a.up {
			a.lift += hover(g.tick, a.name)
		} else {
			drawFeetShadow(screen, fx, fy, a.fog)
		}
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
	stock()
	glow()
	propsUpTo(math.Inf(1))
	if hx, hy := at(g.player.X, g.player.Y); g.hands != "" {
		if g.ghost { // floating with him
			_, hy = at(g.player.X, g.player.Y-hover(g.tick, "Bob"))
		}
		g.drawHands(screen, hx, hy)
	}
	if !g.ghost { // the palette replaces the day and night (D160)
		g.drawDarkness(screen, ox, oy)
	}
	g.drawRequests(screen, ox, oy)
	g.drawTalk(screen, ox, oy)
	g.drawKeepers(screen, ox, oy)
	g.drawVoices(screen, ox, oy)
	g.drawRituals(screen, ox, oy)
	for _, f := range g.floaters { // "+$X" rising over the door (D87); "+N" Ecto in ghost only
		x, y := at(f.x, f.y-(g.now()-float64(f.at))*floaterRisePerFr)
		switch {
		case g.state == StateTalk: // none behind the talk's lines (X2)
		case f.kind == floatMoney || f.kind == floatEcto && g.ghost: // a currency's kind is its Res (wallet.go)
			g.tinted(screen, f.text, x-3*len(f.text), y, resInk(Res(f.kind)), 1)
		}
	}
}

// feetShadow: the dim patch at a living character's feet (F0 Shadows), 14x5 world px.
var feetShadow = func() *ebiten.Image {
	img := image.NewNRGBA(image.Rect(0, 0, 14, 5))
	for y := range 5 {
		for x := range 14 {
			if d := math.Hypot((float64(x)+.5-7)/7, (float64(y)+.5-2.5)/2.5); d < 1 {
				img.SetNRGBA(x, y, color.NRGBA{0, 0, 0, uint8(130 * min(1, 2*(1-d)))})
			}
		}
	}
	return ebiten.NewImageFromImage(img)
}()

// drawFeetShadow lays feetShadow under the feet whose box center is at (x,y), screen px; fading with the fog
// as its character does.
func drawFeetShadow(screen *ebiten.Image, x, y int, fog float64) {
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Scale(zoom, zoom)
	op.GeoM.Translate(float64(x-7*zoom), float64(y-zoom)) // centered on the soles
	op.ColorScale.ScaleAlpha(float32(1 - fog))
	screen.DrawImage(feetShadow, op)
}

// drawPlant draws the plant sprite standing on (x,y), screen px.
func (g *Game) drawPlant(screen *ebiten.Image, sprite string, x, y int) {
	img := g.sprite(sprite)
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Scale(zoom, zoom)
	op.GeoM.Translate(float64(x-img.Bounds().Dx()*zoom/2), float64(y-img.Bounds().Dy()*zoom))
	g.blit(screen, img, op, modeProp, 0, nil, 0)
}

// drawDoors draws each door's leaves slid by its Open amount, clipped to their own cells: a Room door
// slides right; the Entrance's leaves slide apart.
func (g *Game) drawDoors(screen *ebiten.Image, ox, oy int) {
	const ts = world.TileSize
	m := g.hotel.Map
	for _, d := range g.hotel.Doors {
		if d.Room >= 0 && !g.isOwned(d.Room) {
			g.drawBoarded(screen, d, ox, oy)
			continue
		}
		if d.Room >= 0 && g.reeling(d.Room) { // it rolls with its Room's reel
			continue
		}
		off := int(math.Round(d.Open() * ts))
		c0, c1 := m.Cell(d.Rect.Min), m.Cell(d.Rect.Max.Sub(image.Pt(1, 1)))
		for y := c0.Y; y <= c1.Y; y++ {
			for x := c0.X; x <= c1.X; x++ {
				id := m.At(world.LayerDecor, x, y)
				if !m.TS.Tiles[id].Door || off == ts {
					continue
				}
				src, dx := image.Rect(0, 0, ts-off, ts), off // slides right: its left part shows, shifted
				if d.Room < 0 && x == c0.X {
					src, dx = image.Rect(off, 0, ts, ts), 0 // Entrance's left leaf slides left
				}
				op := &ebiten.DrawImageOptions{}
				op.GeoM.Scale(zoom, zoom)
				op.GeoM.Translate(float64((m.Origin(x, y).X+dx)*zoom-ox), float64(y*ts*zoom-oy))
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
