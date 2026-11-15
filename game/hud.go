package game

import (
	"image/color"
	"math"
	"strings"

	"ghosthotel/assets"
	"ghosthotel/world"

	"github.com/hajimehoshi/ebiten/v2"
)

// The HUD (D179): a counter per currency appears when its wallet value changes, rolls to it, slides in
// from the screen's edge (Money from the left, Ecto from the right), holds, then slides back out. Each
// side is a column of notices, centered vertically on what it shows (2026-11-08): a notice coming in or
// going makes the others ease to their new places. Placeholders.
const (
	rollShare   = 0.1     // of the gap a counter closes per frame, at least 1
	holdFrames  = 3 * tps // a counter stays after the last change,
	slideFrames = tps / 3 // then slides out over this; it slides in over the same
	inkPeriod   = 4 * tps // a currency's color cycle (resInk)
	hudScale    = 3       // as big as the request balloon's "!!" (balloonScale)
	hudMargin   = 12      // screen px between a counter and its edge
	noteScale   = 2       // a text notice's size
	slotGap     = 12      // screen px between notices in a column
	slotEase    = 0.15    // of the way to its new place a notice moves per frame
	noticeWrap  = 18      // a text notice's line, in letters: it stays in the screen's side margin
)

// notice is the HUD's text notice (07 U2): the one place for words to the player. The game says none today
// (2026-11-14: no system text, neither a confirmation nor a refusal; show, don't tell), so nothing calls notify
// but the tests; the mechanic stays. It slides in on the left column, holds, and slides out. It is the only place for it: the game has no message
// line, and nothing else may print one (guard_test.go).
type notice struct {
	text string
	on   bool // said and not yet dropped: what the View reads, shown or slid out
	age  int  // frames since it was said
	slot
}

// notify says text on the HUD; "" drops what it was saying.
func (g *Game) notify(text string) {
	if text == "" {
		g.note.on = false
		return
	}
	g.note.text, g.note.on, g.note.age = text, true, 0
}

// notice (08 B1 seam): a brief timed notice in the right-side feed, in ink (green or purple). Any file may call
// it; B1-N builds the feed. Until then it is the HUD's one notice.
func (g *Game) notice(text string, ink color.Color) { g.notify(text) }

// said: what the HUD's notice says, "" once dropped.
func (g *Game) said() string {
	if g.note.on {
		return g.note.text
	}
	return ""
}

// lines: the notice's text, wrapped at its words.
func (n notice) lines() []string {
	var ls []string
	for _, w := range strings.Fields(n.text) {
		if k := len(ls) - 1; k >= 0 && len(ls[k])+1+len(w) <= noticeWrap {
			ls[k] += " " + w
		} else {
			ls = append(ls, w)
		}
	}
	return ls
}

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
	if t := g.nextGuest(); t != "" {
		g.guestText = t
	}
	note := g.nextGuest() != ""
	g.note.age++
	say := g.note.on && g.note.age < holdFrames
	column([]*slot{&g.hud[Money].slot, &g.guestNote, &g.note.slot}, []int{glyphH * hudScale, glyphH * noteScale, len(g.note.lines()) * glyphH * noteScale},
		[]bool{want(Money), note, say})
	g.note.slide(say)
	cue := g.state == StatePlay && g.talker() != nil // TALK (07 X2): SPACE would start the talk
	column([]*slot{&g.hud[Ecto].slot, &g.talkCue}, []int{glyphH * hudScale, glyphH * hudScale}, []bool{want(Ecto), cue})
	g.talkCue.slide(cue)
	g.hud[Money].slide(want(Money))
	g.hud[Ecto].slide(want(Ecto))
	g.guestNote.slide(note)
}

// drawHUD draws each counter that is in: "$N" on the left, a skull and N on the right.
func (g *Game) drawHUD(s *ebiten.Image) {
	if g.skull == nil {
		g.skull = bitmap(skullRows)
	}
	sw := s.Bounds().Dx()
	g.drawBadge(s)
	if n := g.guestNote; n.in > 0 {
		ease := n.in * n.in * (3 - 2*n.in)
		w := len(g.guestText) * glyphW * noteScale
		printAt(s, g.guestText, int(-float64(w)+float64(w+hudMargin)*ease), int(n.y), noteScale, resInk(Money), 1)
	}
	if n := g.note; n.in > 0 { // words alone, never on a background (2026-11-14): white, outlined in black
		ease := n.in * n.in * (3 - 2*n.in)
		ls := n.lines()
		w := 0
		for _, l := range ls {
			w = max(w, len(l)*glyphW*noteScale)
		}
		x := int(-float64(w) + float64(w+hudMargin)*ease)
		for k, l := range ls {
			outline(s, l, x, int(n.y)+k*glyphH*noteScale, noteScale, color.White, 1)
		}
	}
	if c := g.talkCue; c.in > 0 { // TALK, from off the right edge, in the Ectoplasm counter's own style (live test 1)
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
// behind it; off screen it waits at the nearest edge, seen from anywhere.
func (g *Game) drawBadge(s *ebiten.Image) {
	if g.state != StatePlay || g.intro != nil || g.viewT > 0 || !g.newBuy() {
		return
	}
	mw, mh := g.mapSize()
	ox, oy := g.cam.Origin(g.player.X, g.player.Y, mw, mh)
	t := g.hotel.Terminal
	w, h := glyphW*badgeScale, glyphH*badgeScale
	x := min(max(t.Max.X*zoom-ox, hudMargin), s.Bounds().Dx()-w-hudMargin)
	y := min(max((t.Min.Y+world.TileSize+10)*zoom-oy-13*badgeScale, hudMargin), s.Bounds().Dy()-h-hudMargin) // the glyph's foot (row 12) on the screen's top, 10 px into its cell
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
