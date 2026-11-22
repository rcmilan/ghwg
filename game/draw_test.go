package game

import (
	"testing"

	"ghosthotel/world"

	"github.com/hajimehoshi/ebiten/v2"
)

// Live test 5: a floating character in a doorway is cut at the lintel's top, so its head doesn't poke out over the
// wall; inside the Room (feet in the lintel's row) and on the mat it is drawn whole.
func TestLintelClip(t *testing.T) {
	g, _ := headless(t, 2, 1, 1)
	g.startRun()
	m, d := g.hotel.Map, g.hotel.Rooms[0].Door
	l := -1
	for y := d.Y - 1; y <= d.Y+1; y++ {
		if id := m.At(world.LayerWalls, d.X, y); id != 0 && g.ts.Tiles[id].Name == "wall_lintel" {
			l = y
		}
	}
	if l < 0 {
		t.Fatal("no lintel over Room 0's door")
	}
	screen := ebiten.NewImage(640, 360)
	x := float64(d.X*world.TileSize + world.TileSize/2)
	feet := func(row int) float64 { return float64(row*world.TileSize + world.TileSize/2) }
	oy := screenPx(float64(l*world.TileSize)) - 100 // the camera: the lintel's top 100 px down the screen
	if got := g.lintelClip(screen, x, feet(l+1), oy).Bounds().Min.Y; got != 100 {
		t.Errorf("in the doorway: clipped at %d, want the lintel's top, 100", got)
	}
	for _, row := range []int{l, l + 2} {
		if got := g.lintelClip(screen, x, feet(row), oy); got != screen {
			t.Errorf("feet in row %d (lintel %d): clipped to %v, want the whole screen", row, l, got.Bounds())
		}
	}
}
