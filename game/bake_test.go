package game

import (
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
)

// 08 W3 live test: ghost mode fell to 30 FPS on the web, one shader draw per sprite. Its sprites are baked once,
// a few a frame (the first shift lagged baking them all at once), and only images the game keeps are baked: an
// image made per frame would bake every frame (or, as Ebitengine recycles sub-images, every few).
func TestGhostBakesOnce(t *testing.T) {
	g, err := NewGrid(2, 1, 1) // with its sprites, unlike the headless test worlds
	if err != nil {
		t.Fatal(err)
	}
	g.DevBusy()
	for range 3000 {
		g.Update()
	}
	g.ghost = true
	screen := ebiten.NewImage(screenW, screenH)
	g.drawFrame(screen)
	if n := len(g.baked); n == 0 || n > bakeBudget {
		t.Fatalf("the first ghost frame baked %d looks, want 1..%d", n, bakeBudget)
	}
	n, frames := 0, 0
	for ; len(g.baked) != n; frames++ {
		if frames > 100 {
			t.Fatalf("still baking after %d frames (%d looks): an image made per frame reaches blit", frames, len(g.baked))
		}
		n = len(g.baked)
		g.drawFrame(screen)
	}
	t.Logf("%d looks baked over %d frames", n, frames)
	kept := map[*ebiten.Image]bool{g.coverHalves[0]: true, g.coverHalves[1]: true}
	for _, img := range g.tiles {
		kept[img] = true
	}
	for _, img := range g.icons {
		kept[img] = true
	}
	for _, s := range g.characters {
		kept[s[0]], kept[s[1]] = true, true
	}
	for _, p := range g.props {
		kept[p.band] = true
	}
	for k := range g.baked {
		if !kept[k.src] {
			t.Fatalf("baked an image the game does not keep (%v): made per frame?", k.src.Bounds())
		}
	}
}
