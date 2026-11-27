package game

import (
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"

	"ghosthotel/sound"
)

// The pause (07 U1, 09 T407): ESC with nothing open stops the world on Resume / Effects / Quit to title.
// ESC or SPACE on Resume goes on; only SPACE on Quit ends the Run. A / D move the Effects slider; the value
// applies when the pause closes (updateSound), and is never saved. No Music row while the game has no music
// (10, 2026-11-27: the music, #16, is not in the release).
var pauseRows = [...]string{"RESUME", "EFFECTS", "QUIT TO TITLE"}

const pauseEffects = 1 // the slider row, on the Effects bus

// DevPause opens the pause on a Run (-pause), for prints.
func (g *Game) DevPause() { g.state, g.pausePick = StatePause, 0 }

func (g *Game) updatePause() {
	key := g.in.JustPressed
	n := len(pauseRows)
	switch {
	case key(ebiten.KeyW): // wraps, like the Terminal's lists
		g.pausePick = (g.pausePick + n - 1) % n
	case key(ebiten.KeyS):
		g.pausePick = (g.pausePick + 1) % n
	case key(ebiten.KeyA) || key(ebiten.KeyD):
		if g.pausePick == pauseEffects {
			v := &g.snd.vol[sound.Effects]
			if key(ebiten.KeyA) {
				*v = max(*v-1, 0)
			} else {
				*v = min(*v+1, volSteps)
			}
		}
	case key(ebiten.KeySpace) && g.pausePick == n-1:
		g.toTitle()
	case key(ebiten.KeySpace) && g.pausePick == 0:
		g.state = StatePlay
	}
}

func (g *Game) drawPause(s *ebiten.Image) {
	rect(s, 0, 0, screenW, screenH, color.RGBA{0, 0, 0, 150})
	const w, h = 320, 56 + len(pauseRows)*rowH + 8
	x, y := (screenW-w)/2, (screenH-h)/2
	rect(s, x, y, w, h, storeInk)
	rect(s, x+3, y+3, w-6, h-6, storePage)
	g.tintedAt(s, "PAUSED", x+w/2-6*glyphW, y+10, 2, storeInk, 1)
	for k, name := range pauseRows {
		ry := y + 56 + k*rowH
		if k == g.pausePick {
			rect(s, x+12, ry, w-24, rowH-4, storeCard)
			rect(s, x+12, ry, 4, rowH-4, storeAccent)
		}
		if k != pauseEffects {
			g.tintedAt(s, name, x+w/2-len(name)*glyphW, ry+2, 2, storeInk, 1)
			continue
		}
		g.tintedAt(s, name, x+24, ry+2, 2, storeInk, 1)
		const segW, segH, gap = 10, 14, 3 // volSteps segments, filled up to the value
		bx := x + w - 24 - volSteps*(segW+gap) + gap
		for i := range volSteps {
			c := storeCard
			if k == g.pausePick {
				c = storePage // the picked row's card would hide an empty segment
			}
			if i < g.snd.vol[sound.Effects] {
				c = storeAccent
			}
			rect(s, bx+i*(segW+gap), ry+5, segW, segH, storeInk)
			rect(s, bx+i*(segW+gap)+1, ry+6, segW-2, segH-2, c)
		}
	}
}
