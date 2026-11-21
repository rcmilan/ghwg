package game

import (
	"image/color"
	"math"

	"ghosthotel/assets"
	"ghosthotel/world"

	"github.com/hajimehoshi/ebiten/v2"
)

// The HUD (D179): a counter per currency appears when its wallet value changes, rolls to it, slides in
// from the screen's edge (Money from the left, Ecto from the right), holds, then slides back out. Each
// side is a column of notices, centered vertically on what it shows (2026-11-08): a notice coming in or
// going makes the others ease to their new places. Placeholders.
const (
	rollShare    = 0.1     // of the gap a counter closes per frame, at least 1
	holdFrames   = 3 * tps // a counter stays after the last change,
	slideFrames  = tps / 3 // then slides out over this; it slides in over the same
	inkPeriod    = 4 * tps // a currency's color cycle (resInk)
	hudScale     = 3       // as big as the request balloon's "!!" (balloonScale)
	hudMargin    = 12      // screen px between a counter and its edge
	slotGap      = 12      // screen px between notices in a column
	slotEase     = 0.15    // of the way to its new place a notice moves per frame
	noticeFrames = 4 * tps // a feed notice holds this long
	noticeMax    = 3       // and at most this many show
)

// notice is the HUD's feed (08 B1-N): brief timed notices on the right, stacked under the Ecto counter and TALK's
// cue, in the Ecto counter's own text. At most noticeMax show; a newer one sends the oldest out early. No persistent rows. The only
// place for words to the player: the game has no message line, and nothing else may print one (guard_test.go).
type notice struct{ items []feedItem }

type feedItem struct {
	text string
	age  int // frames since it was said
	slot
}

// notice says text in the feed, one line. Any file may call it: the one path.
func (g *Game) notice(text string) {
	if text == "" {
		return
	}
	f := &g.note
	f.items = append(f.items, feedItem{text: text})
	for live := 0; ; {
		live = 0
		for _, it := range f.items {
			if it.age < noticeFrames {
				live++
			}
		}
		if live <= noticeMax {
			break
		}
		for k := range f.items {
			if f.items[k].age < noticeFrames {
				f.items[k].age = noticeFrames
				break
			}
		}
	}
}

// noticesHidden: the open Terminal covers every notice (they keep ticking meanwhile).
func (g *Game) noticesHidden() bool { return g.state == StateTerminal }

// slot: a notice's place in its side column: how far in it is slid, and its top, screen px.
type slot struct{ in, y float64 }

// slide moves it a frame toward in (want) or out.
func (s *slot) slide(want bool) {
	step := 1.0 / slideFrames
	if !want {
		step = -step
	}
	s.in = min(1, max(0, s.in+step))
}

// column places a side's notices, top to bottom: the wanted ones stacked around mid-screen, each
// easing to its place; one just coming in starts there. h: each one's height, screen px.
func column(slots []*slot, h []int, want []bool) {
	total := -slotGap
	for i := range slots {
		if want[i] {
			total += h[i] + slotGap
		}
	}
	y := float64(viewH*zoom/2 - total/2)
	for i, s := range slots {
		if !want[i] {
			continue
		}
		if s.in == 0 {
			s.y = y
		}
		s.y += (y - s.y) * slotEase
		y += float64(h[i] + slotGap)
	}
}

// counter is one currency's number on screen. Its observer is a per-frame diff with the wallet.
type counter struct {
	shown, last Num  // the number drawn; the wallet as of last frame
	age         int  // frames since the last change
	seen        bool // changed at least once this Run: hidden until then
	slot             // in: 0 off screen .. 1 fully in
}

// updateHUD runs each frame: a changed wallet value (earn or spend) brings its counter in again.
func (g *Game) updateHUD() {
	for r := range g.hud {
		c := &g.hud[r]
		if v := g.wallet[r]; v != c.last {
			c.last, c.age, c.seen = v, 0, true
		} else {
			c.age++
		}
		gap := float64(c.last - c.shown)
		if d := max(math.Abs(gap)*rollShare, 1); d >= math.Abs(gap) {
			c.shown = c.last
		} else {
			c.shown += Num(math.Copysign(d, gap))
		}
		if g.state == StateTerminal || g.state == StateView { // the Terminal and the hotel view show both, live, and they hold from its close
			c.age = 0
		}
	}
	want := func(r Res) bool {
		return g.state == StateTerminal || g.state == StateView || g.hud[r].seen && g.hud[r].age < holdFrames
	}
	t := g.nextGuest()
	switch {
	case t == "":
		g.guestAge = 0
	case t != g.guestText: // a new line, or one back after it went: it holds again
		g.guestText, g.guestAge = t, 0
	default:
		g.guestAge++
	}
	note := t != "" && g.guestAge < noticeFrames
	column([]*slot{&g.hud[Money].slot, &g.guestNote}, []int{glyphH * hudScale, glyphH * hudScale}, []bool{want(Money), note})
	cue := g.state == StatePlay && g.talker() != nil // TALK (07 X2): SPACE would open the card
	slots, hs, wants := []*slot{&g.hud[Ecto].slot, &g.talkCue}, []int{glyphH * hudScale, glyphH * hudScale}, []bool{want(Ecto), cue}
	items := g.note.items[:0]
	for _, it := range g.note.items {
		if it.age++; it.age < noticeFrames || it.in > 0 {
			items = append(items, it)
		}
	}
	g.note.items = items
	for k := range items {
		it := &g.note.items[k]
		slots, hs, wants = append(slots, &it.slot), append(hs, glyphH*hudScale), append(wants, it.age < noticeFrames)
	}
	column(slots, hs, wants)
	for k := range items {
		it := &g.note.items[k]
		it.slide(it.age < noticeFrames)
	}
	g.talkCue.slide(cue)
	g.hud[Money].slide(want(Money))
	g.hud[Ecto].slide(want(Ecto))
	g.guestNote.slide(note)
	g.questHud.update(g)
}

// drawHUD draws each counter that is in: "$N" on the left, a skull and N on the right.
func (g *Game) drawHUD(s *ebiten.Image) {
	if g.skull == nil {
		g.skull = bitmap(skullRows)
	}
	sw := s.Bounds().Dx()
	g.drawBadge(s)
	if !g.noticesHidden() {
		if n := g.guestNote; n.in > 0 { // the Money counter's text, from the left edge
			ease := n.in * n.in * (3 - 2*n.in)
			w := len(g.guestText) * glyphW * hudScale
			g.tintedAt(s, g.guestText, int(-float64(w)+float64(w+hudMargin)*ease), int(n.y), hudScale, resInk(Money), 1)
		}
		colW := 0 // the feed's lines all start at one x: the widest live one, against the right margin
		for _, n := range g.note.items {
			colW = max(colW, len(n.text)*glyphW*hudScale)
		}
		for _, n := range g.note.items { // the Ecto counter's text, no outline, no background
			if n.in > 0 {
				ease := n.in * n.in * (3 - 2*n.in)
				x := float64(sw) - float64(colW+hudMargin)*ease
				g.tintedAt(s, n.text, int(x), int(n.y), hudScale, resInk(Ecto), 1)
			}
		}
	}
	if c := g.talkCue; c.in > 0 && !g.noticesHidden() { // TALK, from off the right edge, in the Ectoplasm counter's own style (live test 1)
		ease := c.in * c.in * (3 - 2*c.in)
		w := 4 * glyphW * hudScale
		g.tintedAt(s, "TALK", int(float64(sw)-float64(w+hudMargin)*ease), int(c.y), hudScale, resInk(Ecto), 1)
	}
	for r, c := range g.hud {
		y := int(c.y)
		if c.in == 0 || g.state == StateTerminal { // the Terminal's own bar shows the wallet (08 F0-T)
			continue
		}
		ease := c.in * c.in * (3 - 2*c.in)
		v := fmtNum(c.shown)
		if Res(r) == Money {
			v = "$" + v
		}
		w := len(v) * glyphW * hudScale
		if Res(r) == Ecto {
			w += (len(skullRows[0]) + 2) * hudScale
		}
		travel := float64(w + hudMargin)
		x := int(-float64(w) + travel*ease) // from off the left edge
		if Res(r) == Ecto {
			x = int(float64(sw) - travel*ease) // from off the right edge
			g.stamp(s, g.skull, x, y+skullDY*hudScale, hudScale, resInk(Ecto))
			x += (len(skullRows[0]) + 2) * hudScale
		}
		g.tintedAt(s, v, x, y, hudScale, resInk(Res(r)), 1)
	}
	g.questHUD(s) // 08 W5
	g.drawHeldSlot(s)
}

// fresh: store item i is listed, on sale, affordable and not owned: one the Terminal's "!" can count.
func (g *Game) fresh(i int) bool {
	it := shop[i]
	return it.Tab < tabCart && !g.gone(i) && !g.owned(i) && g.onSale(i) && g.afford(g.priceOf(i)) && (it.Grow == 0 || g.growRefuse(it.Grow) == "")
}

// newBuy: a fresh store item the open Terminal hasn't shown this Run (08 F0: the Terminal's "!").
func (g *Game) newBuy() bool {
	for i, it := range shop {
		if g.fresh(i) && !g.seen[it.ID] {
			return true
		}
	}
	return false
}

const badgeScale = 3 // the "!" over the Terminal's screen, as big as the counters

// drawBadge shows a "!" at the top right of the Terminal's screen while newBuy, in play, clear of the Concierge
// behind it; only where the Terminal is on screen.
func (g *Game) drawBadge(s *ebiten.Image) {
	if g.state != StatePlay || g.intro != nil || g.viewT > 0 || !g.newBuy() {
		return
	}
	mw, mh := g.mapSize()
	ox, oy := g.cam.Origin(g.player.X, g.player.Y, mw, mh)
	t := g.hotel.Terminal
	w, h := glyphW*badgeScale, glyphH*badgeScale
	x := t.Max.X*zoom - ox
	y := (t.Min.Y+world.TileSize+10)*zoom - oy - 13*badgeScale // the glyph's foot (row 12) on the screen's top, 10 px into its cell
	if x < -w || y < -h || x > s.Bounds().Dx() || y > s.Bounds().Dy() {
		return
	}
	outline(s, "!", x, y, badgeScale, resInk(Money), 1)
}

// outline prints v at scale in c, at alpha a, outlined in black: words that stand alone, on no background.
func outline(s *ebiten.Image, v string, x, y int, scale float64, c color.Color, a float32) {
	k := int(scale)
	for _, d := range [...][2]int{{-1, 0}, {1, 0}, {0, -1}, {0, 1}, {-1, -1}, {1, -1}, {-1, 1}, {1, 1}} {
		printAt(s, v, x+d[0]*k, y+d[1]*k, scale, color.Black, a)
	}
	printAt(s, v, x, y, scale, c, a)
}

// The debug font's glyph cell, px; its digits sit on rows 3..12 of the 16.
const glyphW, glyphH = 6, 16

// The Ecto icon (the skull), drawn in the ink of the currency; skullDY lines it up with the digits.
var skullRows = []string{
	"..XXXXX..",
	".XXXXXXX.",
	"XXXXXXXXX",
	"XX..X..XX",
	"XX..X..XX",
	"XXXXXXXXX",
	".XXX.XXX.",
	"..XXXXX..",
	"..X.X.X..",
	"..XXXXX..",
}

const skullDY = 3

// bitmap: white where 'X', clear elsewhere.
func bitmap(rows []string) *ebiten.Image {
	img := ebiten.NewImage(len(rows[0]), len(rows))
	for y, row := range rows {
		for x, ch := range row {
			if ch == 'X' {
				img.Set(x, y, color.White)
			}
		}
	}
	return img
}

// stamp draws a white bitmap at scale, in c, over a black shadow one scaled px down-right.
func (g *Game) stamp(s, img *ebiten.Image, x, y int, scale float64, c color.Color) {
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Scale(scale, scale)
	op.GeoM.Translate(float64(x)+scale, float64(y)+scale)
	op.ColorScale.Scale(0, 0, 0, 1)
	s.DrawImage(img, op)
	op.GeoM.Translate(-scale, -scale)
	op.ColorScale.Reset()
	op.ColorScale.ScaleWithColor(c)
	s.DrawImage(img, op)
}

var (
	gold   = assets.Money
	silver = color.RGBA{206, 212, 226, 255}
	green  = color.RGBA{96, 226, 120, 255}
	purple = color.RGBA{168, 92, 230, 255}
	inks   = [2][2]color.RGBA{Money: {gold, silver}, Ecto: {green, purple}}
)

// inkAt: r's color t frames into its cycle, eased from the first ink to the second at half a period and
// back.
func inkAt(r Res, t int64) color.RGBA {
	w := (1 - math.Cos(2*math.Pi*float64(t)/inkPeriod)) / 2
	a, b := inks[r][0], inks[r][1]
	mix := func(x, y uint8) uint8 { return uint8(math.Round(float64(x) + (float64(y)-float64(x))*w)) }
	return color.RGBA{mix(a.R, b.R), mix(a.G, b.G), mix(a.B, b.B), 255}
}

// tinted prints v in the debug font at 1×, in c at alpha a (no shadow, 2026-11-08).
func (g *Game) tinted(s *ebiten.Image, v string, x, y int, c color.Color, a float32) {
	g.tintedAt(s, v, x, y, 1, c, a)
}

// tintedAt is tinted at a scale.
func (g *Game) tintedAt(s *ebiten.Image, v string, x, y int, scale float64, c color.Color, a float32) {
	printAt(s, v, x, y, scale, c, a)
}

// DevNotices (dev flag -notices): three notices in the feed, to print the stack.
func (g *Game) DevNotices() {
	g.notice("101 CLOSED")
	g.notice("GUESTS FLED 102")
	g.notice("103 CLOSED")
}
