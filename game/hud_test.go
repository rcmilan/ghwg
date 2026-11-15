package game

import (
	"image/color"
	"testing"
)

// T179: a counter is hidden until its first change, rolls up to the wallet and slides in from off screen,
// holds, then slides back out; a spend brings it in again.
func TestCounters(t *testing.T) {
	g, _ := headless(t, 2, 1, 1)
	g.startRun()
	g.step(0, 0)
	if g.hud[Money].in != 0 || g.hud[Ecto].in != 0 {
		t.Fatal("a counter shows at the Run start")
	}
	g.earn(Money, 100, noFloat)
	g.step(0, 0)
	if c := g.hud[Money]; c.in <= 0 || c.in >= 1 || c.shown <= 0 || c.shown >= 100 {
		t.Fatalf("one frame after +100: in %v, shown %v; want sliding in and rolling", c.in, c.shown)
	}
	if g.hud[Ecto].in != 0 {
		t.Fatal("Ecto shows though only Money changed")
	}
	for range tps - 1 {
		g.step(0, 0)
	}
	if c := g.hud[Money]; c.shown != 100 || c.in != 1 {
		t.Fatalf("1 s after +100: shown %v, in %v; want 100, fully in", c.shown, c.in)
	}
	for range holdFrames - tps { // holdFrames frames after the change
		g.step(0, 0)
	}
	if in := g.hud[Money].in; in != 1 {
		t.Fatalf("%d frames after the change: in %v, want still 1", holdFrames, in)
	}
	g.step(0, 0)
	if in := g.hud[Money].in; in <= 0 || in >= 1 {
		t.Fatalf("leaving: in %v, want between 0 and 1", in)
	}
	for range slideFrames {
		g.step(0, 0)
	}
	if in := g.hud[Money].in; in != 0 {
		t.Fatalf("after sliding out: in %v, want 0", in)
	}
	g.pay(Cost{Money: 30})
	g.step(0, 0)
	if c := g.hud[Money]; c.in <= 0 || c.shown >= 100 || c.shown <= 70 {
		t.Fatalf("one frame after paying 30: in %v, shown %v; want sliding in, rolling down", c.in, c.shown)
	}
	for range tps {
		g.step(0, 0)
	}
	if c := g.hud[Money]; c.shown != 70 {
		t.Fatalf("1 s after paying 30: shown %v, want 70", c.shown)
	}
}

// T180: resInk's cycle runs gold → silver → gold for Money and green → purple → green for Ecto.
func TestInkCycle(t *testing.T) {
	for _, c := range []struct {
		r    Res
		t    int64
		want color.RGBA
	}{
		{Money, 0, gold}, {Money, inkPeriod / 2, silver}, {Money, inkPeriod, gold},
		{Ecto, 0, green}, {Ecto, inkPeriod / 2, purple}, {Ecto, inkPeriod, green},
	} {
		if got := inkAt(c.r, c.t); got != c.want {
			t.Errorf("inkAt(%d, %d) = %v, want %v", c.r, c.t, got, c.want)
		}
	}
}

// While the Terminal is open both counters are in and live, even unseen ones; they slide out after it closes.
func TestCountersAtTerminal(t *testing.T) {
	g, _ := headless(t, 2, 1, 1)
	g.startRun()
	g.wallet[Money] = 50
	g.openTerminal()
	for range slideFrames + holdFrames {
		g.updateHUD()
	}
	if m, s := g.hud[Money], g.hud[Ecto]; m.in != 1 || s.in != 1 || m.shown != 50 {
		t.Fatalf("Terminal open: in %v/%v, shown %v; want both in, $50", m.in, s.in, m.shown)
	}
	g.pay(Cost{Money: 20})
	g.updateHUD()
	if c := g.hud[Money]; c.shown == 50 {
		t.Fatal("a spend at the Terminal doesn't roll until it closes")
	}
	g.state = StatePlay
	for range holdFrames + slideFrames {
		g.updateHUD()
	}
	if g.hud[Money].in != 0 || g.hud[Ecto].in != 0 {
		t.Fatal("counters stay after the Terminal closes")
	}
}
