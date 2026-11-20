package game

import (
	"testing"

	"ghosthotel/entities"
)

// quirkRun: a Mirror Run with a settled human in Room 0 and a ghost of Phantasm 4 beside it, both of the quirks.
func quirkRun(t *testing.T, hq, gq Quirk) (*Game, *Party, *Party) {
	t.Helper()
	g, _ := mirrorRun(t, 2, 1)
	h, gp := settledHuman(t, g, 0, 5), ghostOf(g, 0, 4)
	h.Guests[0].Who.Quirk, gp.Guests[0].Who.Quirk = hq, gq
	gp.Guests[0].Haunt = 60
	return g, h, gp
}

// setAsleep moves the clock to a night hour (asleep) or a day hour.
func setAsleep(g *Game, night bool) {
	for asleep(g.clock) != night {
		g.clock += hourFrames
	}
}

func TestQuirkNoneIsNeutral(t *testing.T) {
	g, h, gp := quirkRun(t, NoQuirk, NoQuirk)
	hg, gg := h.Guests[0], gp.Guests[0]
	if g.hauntQuirkMul(0, gg) != 1 || g.sightingQuirkMul(h, hg) != 1 || g.hauntGapQuirkMul(gg) != 1 || g.tipQuirkMul(h) != 1 ||
		g.dirtQuirkMul(h) != 1 || !g.mayFlee(h, hg) || !g.sleeps(h) || !g.mayHaunt(0, gg) || !g.mayPass(gp) || g.hauntGap(gg) != 60 {
		t.Error("NoQuirk changed a rule")
	}
}

func TestQuirkMedium(t *testing.T) {
	g, h, _ := quirkRun(t, Medium, NoQuirk)
	if got := g.sightingQuirkMul(h, h.Guests[0]); got != 2 {
		t.Errorf("Medium sighting x%v, want x2", got)
	}
}

func TestQuirkSkeptic(t *testing.T) {
	g, h, _ := quirkRun(t, Skeptic, NoQuirk)
	guest := h.Guests[0]
	if g.sightingQuirkMul(h, guest) != 0 || g.mayFlee(h, guest) {
		t.Error("a Skeptic's sighting pays or it may flee")
	}
	g.ghost = true
	guest.Fear = guest.FleeAt
	g.scare()
	if h.scares != 0 || h.Fled {
		t.Error("a Skeptic at its FleeAt stepped out")
	}
	guest.Who.Quirk = NoQuirk
	g.scare()
	if h.scares != 1 {
		t.Errorf("control: an ordinary guest at its FleeAt: %d scares, want 1", h.scares)
	}
}

func TestQuirkBigTipper(t *testing.T) {
	g, h, _ := quirkRun(t, BigTipper, NoQuirk)
	if g.tipQuirkMul(h) != 2 {
		t.Error("Big tipper tip not x2")
	}
	g, h, _ = quirkRun(t, NoQuirk, NoQuirk)
	h.Guests = append(h.Guests, &entities.Guest{})
	h.Guests[1].Who.Quirk = BigTipper
	if g.tipQuirkMul(h) != 2 {
		t.Error("a pair with one Big tipper: its mate's tip not x2")
	}
}

func TestQuirkNightOwl(t *testing.T) {
	g, h, _ := quirkRun(t, NightOwl, NoQuirk)
	setAsleep(g, true)
	if g.sleeps(h) {
		t.Error("a Night owl sleeps")
	}
	g.bedtime(h)
	if h.abed {
		t.Error("a Night owl went to bed")
	}
	h.Guests[0].Who.Quirk = NoQuirk
	g.bedtime(h)
	if !h.abed {
		t.Error("control: an ordinary guest did not go to bed at night")
	}
}

// Dirt lands in the Room's Mood twice as hard during a Neat freak's stay, a fright's and a haunt's; the mess it
// leaves at checkout is the next guest's, and is the same as anyone's.
func TestQuirkNeatFreak(t *testing.T) {
	cost := func(q Quirk) (fright, haunt, mess int) {
		g, h, gp := quirkRun(t, q, NoQuirk)
		g.mood[0] = 0
		g.frighten(h, h.Guests[0], 6, 99)
		fright = -g.mood[0]
		g.mood[0] = 0
		g.haunt(0, gp.Guests[0])
		haunt = -g.mood[0]
		g.mood[0], h.mess = 0, 3
		g.leaveMess(h)
		return fright, haunt, -g.mood[0]
	}
	f0, h0, m0 := cost(NoQuirk)
	f1, h1, m1 := cost(NeatFreak)
	if f1 != 2*f0 || h1 != 2*h0 || f0 == 0 || h0 == 0 {
		t.Errorf("fright %d->%d, haunt %d->%d; want twice", f0, f1, h0, h1)
	}
	if m1 != m0 {
		t.Errorf("checkout mess %d->%d, want the same", m0, m1)
	}
}

func TestQuirkShowOff(t *testing.T) {
	g, h, gp := quirkRun(t, NoQuirk, ShowOff)
	if got := g.hauntGap(gp.Guests[0]); got != 30 {
		t.Errorf("Show-off gap %d, want 30 of 60", got)
	}
	if got := g.sightingQuirkMul(h, h.Guests[0]); got != 2 {
		t.Errorf("Show-off sighting x%v, want x2", got)
	}
}

func TestQuirkShy(t *testing.T) {
	g, h, gp := quirkRun(t, NoQuirk, Shy)
	ghost := gp.Guests[0]
	setAsleep(g, false)
	if g.mayHaunt(0, ghost) {
		t.Error("a Shy ghost may haunt while a human is awake")
	}
	g.haunt(0, ghost)
	if ghost.Haunted != 0 || g.mood[0] != moodStart {
		t.Error("a Shy ghost haunted in front of an awake human")
	}
	setAsleep(g, true)
	if !g.mayHaunt(0, ghost) {
		t.Error("a Shy ghost may not haunt a sleeping human")
	}
	h.Guests[0].Who.Quirk = NightOwl // never sleeps: always awake
	if g.mayHaunt(0, ghost) {
		t.Error("a Shy ghost may haunt a Night owl")
	}
}

func TestQuirkJealous(t *testing.T) {
	g, h, gp := quirkRun(t, NoQuirk, Jealous)
	ghost := gp.Guests[0]
	if g.hauntQuirkMul(0, ghost) != 1 {
		t.Error("Jealous x2 for a single guest")
	}
	h.Guests = append(h.Guests, &entities.Guest{X: h.Guests[0].X, Y: h.Guests[0].Y})
	if g.hauntQuirkMul(0, ghost) != 2 {
		t.Error("Jealous not x2 for a Party of two")
	}
}

func TestQuirkGentle(t *testing.T) {
	g, h, gp := quirkRun(t, NoQuirk, Gentle)
	g.haunt(0, gp.Guests[0])
	if g.mood[0] != moodStart {
		t.Errorf("a Gentle haunt cost Mood %d", moodStart-g.mood[0])
	}
	if got := g.sightingQuirkMul(h, h.Guests[0]); got != 0.5 {
		t.Errorf("Gentle sighting x%v, want x1/2", got)
	}
}

// A Homebody neither passes on once spent nor leaves at its Checkout; Father M's sendOver still sends it.
func TestQuirkHomebody(t *testing.T) {
	g, _, gp := quirkRun(t, NoQuirk, Homebody)
	ghost := gp.Guests[0]
	if g.mayPass(gp) {
		t.Fatal("a Homebody may pass")
	}
	ghost.Haunted = ghost.Phantasm
	g.clock = gp.Checkout + 1
	for range 5 {
		g.updateGhost(gp)
	}
	if ghost.PassAt != 0 || gp.state != staying {
		t.Fatalf("a spent Homebody past Checkout: PassAt %d state %d, want it staying", ghost.PassAt, gp.state)
	}
	g.sendOver(0)
	if ghost.PassAt == 0 {
		t.Fatal("Father M could not send a Homebody over")
	}
	g.clock += passFrames
	if g.updateGhost(gp) || !gp.Paid {
		t.Error("a sent-over Homebody did not pass on")
	}
}

func TestPrefersStrongGhost(t *testing.T) {
	_, h, _ := quirkRun(t, Skeptic, NoQuirk)
	if prefersStrongGhost(h) {
		t.Error("an unrevealed Skeptic was read")
	}
	h.revealed = true
	if !prefersStrongGhost(h) {
		t.Error("a revealed Skeptic is not sent to a strong ghost")
	}
	h.Guests[0].Who.Quirk = BigTipper
	if prefersStrongGhost(h) {
		t.Error("a Big tipper was sent to a strong ghost")
	}
}
