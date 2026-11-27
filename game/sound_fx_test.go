package game

import (
	"testing"

	"ghosthotel/sound"
	"ghosthotel/world"
)

// openTestMixer: a mixer and the shipped bank, as OpenSound and loadSound leave them, with no audio context.
func openTestMixer(t *testing.T, g *Game) {
	t.Helper()
	bank, err := sound.LoadBank(sound.Data)
	if err != nil {
		t.Fatal(err)
	}
	g.snd.mix, g.snd.bank = sound.NewMixer(), bank
}

// A ritual's bell: heard near the Priest, fainter away, silent past hearRange.
func TestRitualDing(t *testing.T) {
	g, _, _ := settled(t)
	openTestMixer(t, g)
	pr := &priest{}
	pr.X, pr.Y = g.player.X+hearRange*world.TileSize, g.player.Y
	g.ritualDing(pr)
	if n := g.snd.mix.Playing(); n != 0 {
		t.Fatalf("heard past hearRange: %d playing", n)
	}
	pr.X = g.player.X + world.TileSize
	g.ritualDing(pr)
	if g.snd.mix.Playing() != 1 {
		t.Error("no bell beside the Priest")
	}
}
