package game

import (
	"image/color"
	"math"

	"ghosthotel/assets"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
)

// The HUD (D179): a counter per currency appears when its wallet value changes, rolls to it, slides in
// from the screen's edge at mid-height (Money from the left, Spook from the right), holds, then slides
// back out. Placeholders.
const (
	rollShare   = 0.1     // of the gap a counter closes per frame, at least 1
	holdFrames  = 3 * tps // a counter stays after the last change,
	slideFrames = tps / 3 // then slides out over this; it slides in over the same
	inkPeriod   = 4 * tps // a currency's color cycle (resInk)
	hudScale    = 3       // as big as the request balloon's "!!" (balloonScale)
	hudMargin   = 12      // screen px between a counter and its edge
)

// counter is one currency's number on screen. Its observer is a per-frame diff with the wallet.
type counter struct {
	shown, last Num     // the number drawn; the wallet as of last frame
	age         int     // frames since the last change
	seen        bool    // changed at least once this Run: hidden until then
	in          float64 // 0 off screen .. 1 fully in
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
		if g.state == StateTerminal { // the Terminal shows both, live, and they hold from its close
			c.age = 0
		}
		step := 1.0 / slideFrames
		if g.state != StateTerminal && (!c.seen || c.age >= holdFrames) {
			step = -step
		}
		c.in = min(1, max(0, c.in+step))
	}
}

// drawHUD draws each counter that is in: "$N" on the left, a skull and N on the right.
func (g *Game) drawHUD(s *ebiten.Image) {
	if g.skull == nil {
		g.skull = bitmap(skullRows)
	}
	sw, sh := s.Bounds().Dx(), s.Bounds().Dy()
	y := sh/2 - glyphH*hudScale/2
	for r, c := range g.hud {
		if c.in == 0 {
			continue
		}
		ease := c.in * c.in * (3 - 2*c.in)
		v := fmtNum(c.shown)
		if Res(r) == Money {
			v = "$" + v
		}
		w := len(v) * glyphW * hudScale
		if Res(r) == Spook {
			w += (len(skullRows[0]) + 2) * hudScale
		}
		travel := float64(w + hudMargin)
		x := int(-float64(w) + travel*ease) // from off the left edge
		if Res(r) == Spook {
			x = int(float64(sw) - travel*ease) // from off the right edge
			g.stamp(s, g.skull, x, y+skullDY*hudScale, hudScale, resInk(Spook))
			x += (len(skullRows[0]) + 2) * hudScale
		}
		g.tintedAt(s, v, x, y, hudScale, resInk(Res(r)), 1)
	}
}

// The debug font's glyph cell, px; its digits sit on rows 3..12 of the 16.
const glyphW, glyphH = 6, 16

// The Spook icon (the skull), drawn in the ink of the currency; skullDY lines it up with the digits.
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
	inks   = [2][2]color.RGBA{Money: {gold, silver}, Spook: {green, purple}}
)

// inkAt: r's color t frames into its cycle, eased from the first ink to the second at half a period and
// back.
func inkAt(r Res, t int64) color.RGBA {
	w := (1 - math.Cos(2*math.Pi*float64(t)/inkPeriod)) / 2
	a, b := inks[r][0], inks[r][1]
	mix := func(x, y uint8) uint8 { return uint8(math.Round(float64(x) + (float64(y)-float64(x))*w)) }
	return color.RGBA{mix(a.R, b.R), mix(a.G, b.G), mix(a.B, b.B), 255}
}

// tinted prints v in the debug font at 1×, in c at alpha a, over a black shadow so it reads on any floor.
// text() can't: it's white only.
func (g *Game) tinted(s *ebiten.Image, v string, x, y int, c color.Color, a float32) {
	g.tintedAt(s, v, x, y, 1, c, a)
}

// tintedAt is tinted at a scale.
func (g *Game) tintedAt(s *ebiten.Image, v string, x, y int, scale float64, c color.Color, a float32) {
	if g.ink == nil {
		g.ink = ebiten.NewImage(256, 16)
	}
	g.ink.Clear()
	ebitenutil.DebugPrint(g.ink, v)
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Scale(scale, scale)
	op.GeoM.Translate(float64(x)+scale, float64(y)+scale)
	op.ColorScale.Scale(0, 0, 0, a)
	s.DrawImage(g.ink, op)
	op.GeoM.Translate(-scale, -scale)
	op.ColorScale.Reset()
	op.ColorScale.ScaleWithColor(c)
	op.ColorScale.ScaleAlpha(a)
	s.DrawImage(g.ink, op)
}
