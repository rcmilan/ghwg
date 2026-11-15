package game

import (
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
)

// 08 W3 live test: ghost mode fell to 30 FPS on the web, one shader draw per sprite. Its sprites are baked once,
// and only images the game keeps are baked: an image made per frame would bake every frame (or, as Ebitengine
// recycles sub-images, every few).
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
	n := len(g.baked)
	if n == 0 {
		t.Fatal("a ghost frame baked nothing: still one shader draw per sprite?")
	}
	g.drawFrame(screen)
	if len(g.baked) != n {
		t.Fatalf("a second frame baked %d more looks", len(g.baked)-n)
	}
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
