package game

import (
	"slices"
	"testing"

	"ghosthotel/world"
)

func streetRun(t *testing.T) *Game {
	t.Helper()
	g, _ := headless(t, 2, 1, 1)
	g.startRun()
	return g
}

// 08 W5-STREET: each Dimension sees only its own passers-by, and they leave at the far end.
func TestPassersPerRealm(t *testing.T) {
	g := streetRun(t)
	most := 0
	for range 6000 {
		g.updateStreet()
		most = max(most, len(g.street.walkers))
	}
	if most == 0 || most > 2*maxPassers {
		t.Fatalf("at most %d passers-by at once; want 1..%d", most, 2*maxPassers)
	}
	var real, ghost int
	for _, p := range g.street.walkers {
		if p.ghost {
			ghost++
		} else {
			real++
		}
	}
	for _, in := range [...]bool{false, true} {
		g.ghost = in
		for _, a := range g.streetActors() {
			if a.draw == nil && (a.tint != nil) != in {
				t.Fatalf("ghost view %v draws an actor with tint %v", in, a.tint)
			}
		}
	}
	t.Logf("real %d ghost %d", real, ghost)
}

func TestGossipPool(t *testing.T) {
	has := func(pool, lines []string) bool {
		for _, l := range lines {
			if !slices.Contains(pool, l) {
				return false
			}
		}
		return true
	}
	for _, c := range []struct {
		stage int
		mac   bool
		want  []string
		not   []string
	}{
		{0, false, gossipBefore, gossipEarly},
		{2, false, gossipEarly, gossipBefore},
		{5, false, gossipLate, gossipEarly},
		{5, true, gossipAfter, gossipLate},
	} {
		p := gossipPool(c.stage, c.mac)
		if !has(p, c.want) || !has(p, gossipAny) || slices.Contains(p, c.not[0]) {
			t.Errorf("stage %d mac %v: pool %v", c.stage, c.mac, p)
		}
	}
}

func TestBonusWalkIn(t *testing.T) {
	g := closedRun(t) // no Room owned: nothing fits
	if g.passerTurnsIn() || len(g.parties) != 0 {
		t.Fatal("a passer-by turned in with no Room open")
	}
	g, _ = headless(t, 2, 1, 1)
	g.openAtStart = true
	g.startRun()
	for i := range g.hotel.Rooms {
		if !g.passerTurnsIn() {
			t.Fatalf("turn-in %d refused with a Room Vacant", i)
		}
		for _, p := range g.parties { // always into a Vacant open Room, once each
			n := 0
			for _, q := range g.parties {
				if q.Room == p.Room {
					n++
				}
			}
			if n != 1 {
				t.Fatalf("Room %d has %d Parties", p.Room, n)
			}
		}
		g.clock += 5 // spawnClear: let the last one walk off (it is only at Spawn)
		for _, p := range g.parties {
			for _, gu := range p.Guests {
				gu.Y += 100
			}
		}
	}
	if g.passerTurnsIn() {
		t.Fatal("turned in with every Room taken")
	}
}

// Lines are said only while the Concierge is on the Street, by one passer-by at a time, never a line twice.
func TestStreetLines(t *testing.T) {
	g := streetRun(t)
	for _, out := range [...]bool{false, true} {
		g.street.walkers = nil
		g.player.Y = float64(g.hotel.Spawn.Y)
		if !out {
			g.player.Y = float64(g.hotel.Spawn.Y - 4*world.TileSize)
		}
		spoke := false
		for range 60000 {
			g.updateStreet()
			n, seen := 0, map[string]bool{}
			for _, p := range g.street.walkers {
				if p.sayLeft > 0 {
					n++
					if seen[p.say] {
						t.Fatalf("two passers-by say %q", p.say)
					}
					seen[p.say] = true
				}
			}
			if n > 1 || (!out && n > 0) {
				t.Fatalf("out %v: %d passers-by talking", out, n)
			}
			spoke = spoke || n > 0
		}
		if spoke != out {
			t.Fatalf("out %v: spoke %v", out, spoke)
		}
	}
}

// After a Growth every passer-by is still on the Street, not inside the hotel.
func TestPassersFollowGrowth(t *testing.T) {
	g := streetRun(t)
	for range 3000 {
		g.updateStreet()
	}
	if len(g.street.walkers) == 0 {
		t.Fatal("no passers-by to test")
	}
	if _, err := g.grow(false); err != nil {
		t.Fatal(err)
	}
	g.updateStreet()
	for _, p := range g.street.walkers {
		if d := int(p.Y) - g.hotel.Spawn.Y; d < -world.TileSize || d > world.TileSize {
			t.Fatalf("passer-by at y %.0f, the Street's line is %d", p.Y, g.hotel.Spawn.Y)
		}
	}
}
