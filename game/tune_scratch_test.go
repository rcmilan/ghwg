package game

import (
	"sync"
	"testing"
)

// scratch: X2 tuning, deleted before handing back.
func TestTuneScratch(t *testing.T) {
	const n = 12
	for _, grid := range [][2]int{{2, 1}, {2, 3}} {
		for _, name := range []string{"goals", "matcher", "asker"} {
			rs := make([]Result, n)
			earned := make([][2]Num, n)
			var wg sync.WaitGroup
			for i := range rs {
				wg.Add(1)
				go func() {
					defer wg.Done()
					s, _ := newSim(Config{Cols: grid[0], Rows: grid[1], Days: 4, Seed: uint64(1 + i), Name: name, Strategy: Strategies[name], React: 30, KeyGap: 6})
					for !s.done() && s.frame() {
					}
					rs[i], earned[i] = s.finish(), s.g.earned
				}()
			}
			wg.Wait()
			var money, fled, off, ecto, fails, rooms, em float64
			for i, r := range rs {
				for _, m := range r.MoneyByDay {
					money += float64(m) / 4 / n
				}
				em += float64(earned[i][Money]) / 4 / n
				fled += float64(r.Fled) / n
				off += float64(r.Offered) / n
				ecto += float64(r.EctoEarned) / n
				fails += float64(len(r.Failures))
				rooms += float64(r.Rooms) / n
			}
			t.Logf("%dx%d %-8s net $/day %6.1f  earned $/day %6.1f  fled %4.1f  offered $%4.0f  ecto %4.0f  rooms %.1f  score(earned+ecto/2 a day) %6.1f fails %v", grid[0], grid[1], name, money, em, fled, off, ecto, rooms, em+ecto/8, fails)
		}
	}
}
