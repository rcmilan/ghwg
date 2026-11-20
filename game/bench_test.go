package game

import (
	"fmt"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
)

// The frame budget (P1-F): a busy 6x6 hotel updates in about 2 ms a frame as wasm, so T (x4) fits in ~8 ms
// of the 16.7 ms a frame; 8x8 gets 4 ms (08 W3.5), 10x10 is recorded, with no budget. Run natively with
// go test ./game -bench Update -run '^$', as wasm with tools/wasmtest.sh (its budget check is the wasm one).
const wasmUpdateBudgetMs = 2.0    // the aim; the test fails 25% over it, since "about 2 ms" must hold on a busy machine too
const wasmUpdateBudget8x8Ms = 4.0 // the same, for 8x8

// busyGame: a cols x rows hotel in the busy state (busy.go) with everyone already walking.
func busyGame(tb testing.TB, cols, rows int) *Game {
	g, err := newWorld(cols, rows, 1)
	if err != nil {
		tb.Fatal(err)
	}
	g.DevBusy()
	for range 3000 { // the Parties arrive minutes apart: let them fill the hotel
		g.Update()
	}
	return g
}

func BenchmarkUpdate(b *testing.B) {
	for _, n := range [...]int{3, 6, 8, 10} {
		b.Run(fmt.Sprintf("%dx%d", n, n), func(b *testing.B) {
			g := busyGame(b, n, n)
			b.ReportAllocs()
			b.ResetTimer()
			for range b.N {
				g.Update()
			}
		})
	}
}

// TestUpdateBudget: only as wasm, where the budget applies (native is ~4x faster). A check, not a gate on a
// loaded machine, so it takes the best of five 200-frame runs.
func TestUpdateBudget(t *testing.T) {
	if runtime.GOOS != "js" || testing.Short() {
		t.Skip("the budget is for wasm")
	}
	for _, c := range [...]struct {
		n      int
		budget float64
	}{{6, wasmUpdateBudgetMs}, {8, wasmUpdateBudget8x8Ms}} {
		g := busyGame(t, c.n, c.n)
		best := time.Hour
		for range 5 {
			start := time.Now()
			for range 200 {
				g.Update()
			}
			best = min(best, time.Since(start)/200)
		}
		ms := float64(best) / 1e6
		t.Logf("%dx%d busy: %.2f ms an Update as wasm (budget %.1f)", c.n, c.n, ms, c.budget)
		if ms > 1.25*c.budget {
			t.Errorf("%dx%d Update takes %.2f ms as wasm, budget %.1f", c.n, c.n, ms, c.budget)
		}
	}
}

// BenchmarkDraw: the CPU side of one drawn frame of a busy level-1 4x4 hotel (L2: level 2, 5x5, and its hotel view, T381 refine 7) (08 W3: shifts fell to 8 FPS and a
// growth in ghost crashed the browser; the GPU side shows only in a browser, sh tools/webprof). Run with
// -benchtime 200x: off the game loop, the draw commands are never flushed.
func BenchmarkDraw(b *testing.B) {
	for _, c := range [...]struct {
		name string
		set  func(g *Game)
	}{
		{"real", func(g *Game) {}},
		{"ghost", func(g *Game) { g.ghost = true }},
		{"shift", func(g *Game) { g.startShift() }},
		{"grow-ghost", func(g *Game) { g.ghost = true; g.DevGrow("floor") }},
		{"L2-real", func(g *Game) {}},
		{"L2-ghost", func(g *Game) { g.ghost = true }},
		{"L2-view", func(g *Game) { g.state, g.viewT, g.termDepth = StateView, 1, 1 }},
	} {
		b.Run(c.name, func(b *testing.B) {
			g, err := NewGrid(2, 1, 1) // with its sprites, unlike busyGame's headless world
			if err != nil {
				b.Fatal(err)
			}
			g.DevLevel(1)
			if strings.HasPrefix(c.name, "L2") {
				g.DevLevel(2)
			}
			if c.name == "grow-ghost" { // a 4x3 hotel growing its last floor, as in the live test
				g.Start(false)
				g.DevGrow("col,floor,col,floor")
			} else {
				g.DevBusy()
				for range 3000 {
					g.Update()
				}
			}
			c.set(g)
			g.Layout(0, 0)
			screen := ebiten.NewImage(screenW, screenH)
			for range 60 { // past the first frames' bakes
				g.Draw(screen)
			}
			b.ReportAllocs()
			b.ResetTimer()
			for range b.N {
				g.Draw(screen)
			}
		})
	}
}
