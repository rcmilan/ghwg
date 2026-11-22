package game

import (
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
)

// 08 W5-SCENE: the credits, on black, after the finale's first time: the credits (circle.go), then the epilogue and a
// CONTINUE prompt; SPACE turns the page, then a normal prestige (startPrestige).
const (
	creditsPage = 600 // frames the credits stay before the epilogue comes by itself
	creditsHold = 150 // frames of the epilogue before CONTINUE shows
	creditsFade = 60
)

// stepCredits runs the credits a frame. Space is only read after the first, so the key that started them is not one.
func (g *Game) stepCredits() {
	s := &g.scene
	space := s.t > 0 && g.in.JustPressed(ebiten.KeySpace)
	if s.t++; s.page == 0 && (s.t >= creditsPage || space) {
		s.page, s.t = 1, 0
		return
	}
	if s.page == 1 && s.t >= creditsHold && (space || g.auto != nil) { // a Strategy playing never presses it
		from := s.from
		g.closeScene(from)
		if from == "finale" {
			g.startPrestige() // a normal prestige; the MacGuffin books itself in the next Run's first Growth (macguffinIn)
		}
	}
}

func (g *Game) drawCredits(screen *ebiten.Image) {
	screen.Fill(color.Black)
	s := &g.scene
	a := float32(ease(float64(s.t) / creditsFade))
	centre := func(v string, y int, scale float64, c color.Color, a float32) {
		printAt(screen, v, (screenW-len(v)*glyphW*int(scale))/2, y, scale, c, a)
	}
	if s.page == 0 {
		scale := func(i int) int { return 2 + b2i(i == 0) } // the title larger
		step := func(i int, l string) int {                 // the line's height and the gap after it
			if l == "" { // a blank is a gap, not a line, so the whole fits the screen
				return 12
			}
			return glyphH*scale(i) + 8
		}
		h := -8 // no gap after the last line
		for i, l := range credits {
			h += step(i, l)
		}
		y := (screenH - h) / 2 // centred on the screen
		for i, l := range credits {
			centre(l, y, float64(scale(i)), color.White, a)
			y += step(i, l)
		}
		return
	}
	y := 190
	for _, l := range wrapAt(creditsEpilogue, 48) {
		centre(l, y, 2, color.White, a)
		y += glyphH*2 + 8
	}
	if s.t >= creditsHold && s.t/30%2 == 0 {
		centre("[ SPACE ]  CONTINUE", 400, 2, resInk(Ecto), 1)
	}
}

// DevCredits is main's -credits: the credits, opened alone.
func (g *Game) DevCredits() { g.scene = sceneState{id: "credits"} }
