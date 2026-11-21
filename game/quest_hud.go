package game

import (
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
)

// 08 W5: the questline's checklist. The beat's title heads each side that has a row; the real rows stand top left and
// show only in real, the ghost rows top right and show only in ghost. Done rows are ticked and dimmed. Words on no
// background, outlined, clear of the counters (mid-screen) and the feed (under the Ecto counter).
const (
	questRowScale = 2
	questRowGap   = 26  // screen px from one row's top to the next
	questTitleGap = 34  // from the title's top to the first row's
	questDimmed   = 0.5 // a done row's alpha
)

// questHUD draws the checklist: real rows on the left in real, ghost rows on the right in ghost.
func (g *Game) questHUD(dst *ebiten.Image) {
	if g.state != StatePlay || g.intro != nil || g.quest.Beat < 1 || g.quest.Beat > beatCount {
		return
	}
	bt, side := beats[g.quest.Beat-1], g.realm()
	var rows []objective
	var done []bool
	for i, o := range bt.Objs {
		if o.Side == side {
			rows, done = append(rows, o), append(done, i < len(g.quest.Done) && g.quest.Done[i])
		}
	}
	if len(rows) == 0 {
		return
	}
	sw := dst.Bounds().Dx()
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
	at(bt.Title, len(bt.Title)*glyphW*hudScale, hudMargin, hudScale, ink, 1)
	y := hudMargin + questTitleGap
	for i, o := range rows {
		text, a := "[ ] "+o.Row, float32(1)
		if done[i] {
			text, a = "[X] "+o.Row, questDimmed
		}
		at(text, len(text)*glyphW*questRowScale, y, questRowScale, color.White, a)
		y += questRowGap
	}
}
