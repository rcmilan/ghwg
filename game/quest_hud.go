package game

import (
	"fmt"
	"image/color"

	"ghosthotel/assets"

	"github.com/hajimehoshi/ebiten/v2"
)

// 08 W5 (R1 GUIDE): the questline's checklist. The beat's title heads it; each row is an open objective's next step
// (circle.go objSteps) behind a small icon for the Dimension it is done in (both for the Terminal), a done row is
// ticked and dimmed. The list sits top left in real and top right in ghost, clear of the counters (mid-screen) and the
// feed (under the Ecto counter). Words on no background, outlined.
const (
	questRowScale = 2
	questRowGap   = 26      // screen px from one row's top to the next
	questTitleGap = 60      // from the title's top to the first row's: a line of air between them
	questHold     = 5 * tps // frames the checklist stays after it changes (real time: updateHUD counts real frames)
	questFade     = tps / 2 // and its fade out
	questDimmed   = 0.5     // a done row's alpha
	questIcon     = 7       // an icon's width in its px (drawn at questRowScale)
)

// The Dimension icons and the tick, in white, drawn in the Dimension's ink.
var (
	realRows  = []string{"..XXX..", ".XXXXX.", ".XXXXX.", "..XXX..", ".XXXXX.", "XXXXXXX", "XXXXXXX"}
	ghostRows = []string{"..XXX..", ".XXXXX.", "XX.X.XX", "XXXXXXX", "XXXXXXX", "XXXXXXX", "X.X.X.X"}
	tickRows  = []string{"......X", ".....XX", "X...XX.", "XX.XX..", ".XXX...", "..X...."}
)

// hudIcon: a cached white bitmap by name.
func (g *Game) hudIcon(name string, rows []string) *ebiten.Image {
	if g.icons == nil {
		g.icons = map[string]*ebiten.Image{}
	}
	if img, ok := g.icons[name]; ok {
		return img
	}
	img := bitmap(rows)
	g.icons[name] = img
	return img
}

// questShow times the checklist: shown for questHold after it changes (a beat, an objective, a step's text, the
// Dimension), always in the hotel view, else gone.
type questShow struct {
	sig string
	age int
	img *ebiten.Image // the list drawn whole, to fade as one
}

func (q *questShow) update(g *Game) {
	title, rows := g.guideRows()
	sig := fmt.Sprint(g.realm(), title)
	for _, r := range rows {
		sig += fmt.Sprint("|", r.Text, r.Done)
	}
	if sig != q.sig {
		q.sig, q.age = sig, 0
	} else {
		q.age++
	}
}

// alpha: 1 held, fading to 0 after.
func (q *questShow) alpha(g *Game) float32 {
	if g.state == StateView {
		return 1
	}
	return float32(min(1, max(0, float64(questHold+questFade-q.age)/questFade)))
}

// questHUD draws the checklist while it is to show.
func (g *Game) questHUD(dst *ebiten.Image) {
	a := g.questHud.alpha(g)
	if g.state != StatePlay && g.state != StateView || g.intro != nil || g.quest.Beat > beatCount || a == 0 {
		return
	}
	if a == 1 {
		g.drawQuest(dst)
		return
	}
	q := &g.questHud
	if q.img == nil || q.img.Bounds() != dst.Bounds() {
		q.img = ebiten.NewImage(dst.Bounds().Dx(), dst.Bounds().Dy())
	}
	q.img.Clear()
	g.drawQuest(q.img)
	op := &ebiten.DrawImageOptions{}
	op.ColorScale.ScaleAlpha(a)
	dst.DrawImage(q.img, op)
}

func (g *Game) drawQuest(dst *ebiten.Image) {
	title, rows := g.guideRows()
	side := g.realm()
	sw := dst.Bounds().Dx()
	const pad = 2*questIcon*questRowScale + 4                                    // two icons wide: the text starts at one x
	at := func(text string, w, y int, scale float64, c color.Color, a float32) { // flush left in real, right in ghost
		x := hudMargin
		if side == Ghost {
			x = sw - hudMargin - w
		}
		outline(dst, text, x, y, scale, c, a)
	}
	ink := resInk(Money)
	if side == Ghost {
		ink = resInk(Ecto)
	}
	y := hudMargin
	if title != "" {
		at(title, len(title)*glyphW*hudScale, y, hudScale, ink, 1)
		y += questTitleGap
	}
	for _, r := range rows {
		w := pad + len(r.Text)*glyphW*questRowScale
		x := hudMargin
		if side == Ghost {
			x = sw - hudMargin - w
		}
		if r.Done {
			g.stamp(dst, g.hudIcon("hud-tick", tickRows), x, y+4, questRowScale, color.RGBA{150, 150, 150, 255})
		} else {
			ix := x
			if r.Step.Dim == Human || r.Step.Any {
				g.stamp(dst, g.hudIcon("hud-real", realRows), ix, y+2, questRowScale, markGold)
				ix += questIcon*questRowScale + 2
			}
			if r.Step.Dim == Ghost || r.Step.Any {
				g.stamp(dst, g.hudIcon("hud-ghost", ghostRows), ix, y+2, questRowScale, markSilver)
			}
		}
		a := float32(1)
		if r.Done {
			a = questDimmed
		}
		outline(dst, r.Text, x+pad, y, questRowScale, color.White, a)
		y += questRowGap
	}
}

// ---- the carried word or thing: a small pixel icon, no label ----

// heldRows: the icons' pixels, K the dark outline. A scroll with a speech tail (a word), a key, a trolley handle.
var heldRows = map[string][]string{
	"word": {
		".KKKKKKKKK.",
		"KPPPPPPPPPK",
		"KPLLLLLLLPK",
		"KPPPPPPPPPK",
		"KPLLLLLPPPK",
		"KPPPPPPPPPK",
		"KPLLLLLLLPK",
		".KKKKKKKKK.",
		"..KPK......",
		"...K.......",
	},
	"key": {
		"..KKK.......",
		".KGGGK......",
		"KGGKGGKKKKKK",
		"KGGKGGGGGGGK",
		"KGGGGGKGKGKK",
		".KGGGKKKKKK.",
		"..KKK.......",
	},
	"haft": {
		"KKKKKKKKKKK",
		"KRRRRRRRRRK",
		"KKKKKKKKKKK",
		"...KSSSK...",
		"...KSSSK...",
		"...KSSSK...",
		"...KSSSK...",
		"...KSSSK...",
		"...KKKKK...",
	},
}

var heldPal = map[rune]color.RGBA{
	'K': {26, 20, 30, 255}, 'P': {240, 228, 196, 255}, 'L': {150, 110, 80, 255},
	'G': {244, 196, 60, 255}, 'R': {170, 70, 60, 255}, 'S': {176, 186, 204, 255},
}

// heldIcon: the picture of the word or thing held (verbs.held).
func (g *Game) heldIcon(held string) *ebiten.Image {
	if held == "O8" { // the shop's wreath
		return g.sprite("icon_wreath")
	}
	kind := map[string]string{"O1": "word", "O2": "word", "O9": "key", "O11": "haft"}[held]
	rows := heldRows[kind]
	if rows == nil {
		return nil
	}
	if g.icons == nil {
		g.icons = map[string]*ebiten.Image{}
	}
	if img, ok := g.icons["held-"+kind]; ok {
		return img
	}
	img := ebiten.NewImage(len(rows[0]), len(rows))
	for y, row := range rows {
		for x, ch := range row {
			if c, ok := heldPal[ch]; ok {
				img.Set(x, y, c)
			}
		}
	}
	g.icons["held-"+kind] = img
	return img
}

// drawHeldSlot: the fixed corner slot (bottom left, an icon, no words: the Hotel view's hint has the bottom right) showing the word or thing carried, over everything but the
// Terminal: in play, the hotel view, and an open Card.
func (g *Game) drawHeldSlot(dst *ebiten.Image) {
	if g.verbs.held == "" || g.state == StateTerminal || g.state == StateTitle || g.intro != nil {
		return
	}
	img := g.heldIcon(g.verbs.held)
	if img == nil {
		return
	}
	const sc = 3
	b := img.Bounds()
	w, h := b.Dx()*sc+14, b.Dy()*sc+14
	x, y := hudMargin, dst.Bounds().Dy()-hudMargin-h
	rect(dst, x-2, y-2, w+4, h+4, assets.Night)
	rect(dst, x, y, w, h, color.RGBA{44, 36, 58, 255})
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Scale(sc, sc)
	op.GeoM.Translate(float64(x+7), float64(y+7))
	dst.DrawImage(img, op)
}
