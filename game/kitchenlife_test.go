package game

import (
	"image"
	"testing"

	"ghosthotel/world"
)

// lifeRun: a Run with two Housekeepers idling, stepped for days game days; what the sim ended with, and the
// visits and lines seen on the way.
func lifeRun(t *testing.T, off bool, days int) (money Num, rng uint64, visits, lines int) {
	t.Helper()
	kitchenLifeOff = off
	defer func() { kitchenLifeOff = false }()
	g, _ := headless(t, 2, 1, 7)
	g.startRun()
	g.DevStaff(2, 100000)
	said := map[*housekeeper]int{}
	for range days * dayFrames {
		g.step(0, 0)
		for _, k := range g.keepers {
			for i := range lifeProps {
				if k.job == resting && image.Pt(int(k.X), int(k.Y)).Div(world.TileSize) == g.lifeStand(i).Div(world.TileSize) && !k.unpaid {
					visits++
					break
				}
			}
			if k.talking(g.tick) && said[k] != k.said.at {
				said[k] = k.said.at
				lines++
			}
		}
	}
	return g.wallet[Money], g.rng.Uint64(), visits, lines
}

// 08 W5: idle Housekeepers go to the props and say their lines.
func TestKitchenLife(t *testing.T) {
	_, _, v0, _ := lifeRun(t, true, 3)
	_, _, v1, l1 := lifeRun(t, false, 3)
	if v1 == 0 || l1 == 0 || v1 < 10*v0 {
		t.Fatalf("in 3 days: %d frames at a prop (%d with it off), %d lines; want many, few, some", v1, v0, l1)
	}
}

// The feature draws on none of the sim's streams: its rolls and its lines leave g.rng, g.chat, g.words and g.ident
// where they were. (Where a Housekeeper stands moves job timing, so a Run's money is not the same with it off: that
// is the walk itself, not a draw.)
func TestKitchenLifeStreams(t *testing.T) {
	a, _ := headless(t, 2, 1, 7)
	b, _ := headless(t, 2, 1, 7)
	for _, g := range []*Game{a, b} {
		g.startRun()
		g.DevStaff(2, 100000)
	}
	for range 500 {
		a.clock += 37
		for _, k := range a.keepers {
			a.lifeVisit(k)
			a.lifeSpeak(k)
		}
	}
	if a.rng.Uint64() != b.rng.Uint64() || a.chat.Uint64() != b.chat.Uint64() || a.words.Uint64() != b.words.Uint64() || a.ident.Uint64() != b.ident.Uint64() {
		t.Fatal("a stream moved")
	}
}

// One Housekeeper talks at a time: with another's line showing, it keeps its own for its turn.
func TestKitchenLifeOneAtATime(t *testing.T) {
	g, _ := headless(t, 2, 1, 7)
	g.startRun()
	g.DevStaff(2, 100000)
	a, b := g.keepers[0], g.keepers[1]
	a.said = line{0, "hi", g.tick}
	b.X, b.Y, b.lifeAt, b.lifeLine = 5, 5, image.Pt(5, 5), kitchenAny
	g.lifeSpeak(b)
	if b.said.text != "" || b.lifeLine == "" {
		t.Fatal("it talked over the other")
	}
	a.said = line{}
	g.lifeSpeak(b)
	if b.said.text != kitchenAny || !b.talking(g.tick+lifeFrames-1) || b.talking(g.tick+lifeFrames) {
		t.Fatalf("its line: %q, should last %d frames", b.said.text, lifeFrames)
	}
}
