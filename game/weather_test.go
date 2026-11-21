package game

import "testing"

// Weather is rolled apart from the sim's streams, the same each time for a day, and not always clear.
func TestWeatherApart(t *testing.T) {
	a, _ := headless(t, 2, 1, 7)
	b, _ := headless(t, 2, 1, 7)
	kinds := map[int]bool{}
	for d := 1; d <= 60; d++ {
		k := a.weatherOn(d)
		if k != a.weatherOn(d) {
			t.Fatalf("day %d rolled twice differently", d)
		}
		kinds[k] = true
	}
	if !kinds[wxClear] || len(kinds) < 3 {
		t.Errorf("60 days gave %v", kinds)
	}
	for _, c := range [...][2]uint64{{a.rng.Uint64(), b.rng.Uint64()}, {a.chat.Uint64(), b.chat.Uint64()}, {a.words.Uint64(), b.words.Uint64()}, {a.ident.Uint64(), b.ident.Uint64()}} {
		if c[0] != c[1] {
			t.Fatal("rolling weather moved a stream the sim depends on")
		}
	}
}

func TestStormUntilDawn(t *testing.T) {
	g := streetRun(t)
	g.clock = 14*hourFrames + dayFrames
	if g.stormOn() {
		t.Fatal("a storm before the jar")
	}
	g.summonStorm()
	g.clock += 17 * hourFrames // 07:00 next day
	if !g.stormOn() {
		t.Fatal("the storm ended before dawn")
	}
	g.clock += hourFrames // 08:00
	if g.stormOn() {
		t.Fatal("the storm outlasted dawn")
	}
}
