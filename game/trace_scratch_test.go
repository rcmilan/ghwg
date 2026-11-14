package game

import (
	"testing"
)

// scratch: X2 trace, deleted before handing back.
func TestTraceScratch(t *testing.T) {
	for _, name := range []string{"matcher"} {
		for seed := uint64(1); seed <= 3; seed++ {
			s, _ := newSim(Config{Cols: 2, Rows: 1, Days: 4, Seed: seed, Name: name, Strategy: Strategies[name], React: 30, KeyGap: 6})
			g := s.g
			bad := map[int]bool{}
			fled := map[*Party]bool{}
			seen := map[*Party]partyState{}
			for !s.done() && s.frame() {
				for i := range g.hotel.Rooms {
					b := g.pairing(i) == badPair
					if b != bad[i] {
						fix := Num(0)
						tg := 0
						if p := g.humanIn(i); p != nil {
							_, _, fix, _ = g.sureOffer(p)
							tg = len(g.targets(p))
						}
						t.Logf("%d %s room %d bad=%v fix $%v targets %d money %v", seed, clockText(g.clock), i, b, fix, tg, g.wallet[Money])
						bad[i] = b
					}
				}
				for _, p := range g.parties {
					if p.Realm != Human {
						continue
					}
					if prev := seen[p]; p.state == moving && prev != moving {
						t.Logf("%d %s   MOVE to %d", seed, clockText(g.clock), p.Room)
					}
					seen[p] = p.state
					if p.Fled && !fled[p] {
						fled[p] = true
						t.Logf("%d %s   FLED room %d nerve %d ghost %d", seed, clockText(g.clock), p.Room, partyNerve(p), g.roomPhantasm(p.Room))
					}
				}
			}
		}
	}
}
