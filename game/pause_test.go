package game

import (
	"testing"

	"github.com/hajimehoshi/ebiten/v2"

	"ghosthotel/sound"
)

// T407: W/S wrap over the three rows (no Music row: 10), A/D move only the Effects slider (clamped
// 0..volSteps), SPACE acts on Resume and Quit and does nothing on the slider.
func TestPauseMenu(t *testing.T) {
	g, s := headless(t, 2, 1, 1)
	g.startRun()
	keys(g, s, ebiten.KeyEscape)
	if g.state != StatePause || g.pausePick != 0 {
		t.Fatalf("state %d, pick %d; want the pause on Resume", g.state, g.pausePick)
	}
	keys(g, s, ebiten.KeyA, ebiten.KeyD)
	if g.snd.vol != [2]int{volDefault, volDefault} {
		t.Fatalf("A/D on Resume moved a slider: %v", g.snd.vol)
	}
	keys(g, s, ebiten.KeyS)
	if g.pausePick != pauseEffects {
		t.Fatalf("S: pick %d, want Effects", g.pausePick)
	}
	keys(g, s, ebiten.KeyA)
	if g.snd.vol[sound.Effects] != volDefault-1 {
		t.Fatalf("A on Effects: %v", g.snd.vol)
	}
	for range volSteps + 2 {
		keys(g, s, ebiten.KeyA)
	}
	if g.snd.vol[sound.Effects] != 0 {
		t.Fatalf("Effects clamps at 0, got %d", g.snd.vol[sound.Effects])
	}
	for range volSteps + 2 {
		keys(g, s, ebiten.KeyD)
	}
	if g.snd.vol != [2]int{volDefault, volSteps} {
		t.Fatalf("D on Effects clamps at volSteps, never touches Music: %v", g.snd.vol)
	}
	keys(g, s, ebiten.KeySpace)
	if g.state != StatePause {
		t.Fatal("SPACE on Effects left the pause")
	}
	keys(g, s, ebiten.KeyS, ebiten.KeyS)
	if g.pausePick != 0 {
		t.Fatalf("S past Quit: pick %d, want it wrapped to Resume", g.pausePick)
	}
	keys(g, s, ebiten.KeySpace)
	if g.state != StatePlay {
		t.Fatalf("SPACE on Resume: state %d", g.state)
	}
	keys(g, s, ebiten.KeyEscape, ebiten.KeyW, ebiten.KeySpace)
	if g.state != StateTitle {
		t.Fatalf("SPACE on Quit: state %d, want the title", g.state)
	}
}
