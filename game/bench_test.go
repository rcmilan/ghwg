package game

import (
	"fmt"
	"runtime"
	"testing"
	"time"
)

// The frame budget (P1-F): a busy 6x6 hotel updates in about 2 ms a frame as wasm, so T (x4) fits in ~8 ms
// of the 16.7 ms a frame. 8x8 is recorded, with no budget. Run natively with
// go test ./game -bench Update -run '^$', as wasm with tools/wasmtest.sh (its budget check is the wasm one).
const wasmUpdateBudgetMs = 2.0 // the aim; the test fails 25% over it, since "about 2 ms" must hold on a busy machine too

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
	for _, n := range [...]int{3, 6, 8} {
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
	g := busyGame(t, 6, 6)
	best := time.Hour
	for range 5 {
		start := time.Now()
		for range 200 {
			g.Update()
		}
		best = min(best, time.Since(start)/200)
	}
	ms := float64(best) / 1e6
	t.Logf("6x6 busy: %.2f ms an Update as wasm (budget %.1f)", ms, wasmUpdateBudgetMs)
	if ms > 1.25*wasmUpdateBudgetMs {
		t.Errorf("6x6 Update takes %.2f ms as wasm, budget %.1f", ms, wasmUpdateBudgetMs)
	}
}
