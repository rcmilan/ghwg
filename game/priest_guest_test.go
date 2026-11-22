package game

import (
	"slices"
	"strings"
	"testing"
)

// bookings runs n solo bookings into a fresh hotel (Rooms emptied between) and counts the Priest guests.
func priestBookings(g *Game, n int) (k, m int) {
	for range n {
		g.parties = nil
		p := g.arrive(0)
		if s, ok := guestFather(p); ok {
			if s {
				m++
			} else {
				k++
			}
		}
	}
	return
}

func TestPriestGuestOnlyWhenUnlocked(t *testing.T) {
	g, _ := headless(t, 2, 1, 1)
	g.startRun()
	if k, m := priestBookings(g, 400); k+m != 0 {
		t.Fatalf("%d Priest guests with both calls locked", k+m)
	}
	g.unlock(unlockK)
	if k, m := priestBookings(g, 400); k+m != 0 {
		t.Fatalf("K unlocked only: %d K, %d M; Father K never books", k, m)
	}
	g.unlock(unlockM)
	if k, m := priestBookings(g, 400); k != 0 || m == 0 {
		t.Fatalf("both unlocked: %d K, %d M", k, m)
	}
	setQuest(g, 4) // NIGHT SHIFT: O6 needs his call, so he does not book
	if k, m := priestBookings(g, 400); k+m != 0 {
		t.Fatalf("O6 open: %d K, %d M", k, m)
	}
}

func TestOneOfEachFatherBothWays(t *testing.T) {
	g, _ := headless(t, 2, 1, 1)
	g.startRun()
	g.unlock(unlockM)
	g.wallet[Ecto] = 1000
	p := g.arrive(0)
	g.becomeFather(p, true)
	if !g.fatherIn(true) || g.noFather(1, true) == "" || g.callPriest(1) == "" {
		t.Fatal("Father M can be called while he is a guest")
	}
	if g.noFather(1, false) != "" {
		t.Fatal("Father K is blocked by Father M")
	}
	g.parties = nil
	g.sendPriest(1, true)
	if priestBookings2(g) != 0 {
		t.Fatal("a called Father M booked as a guest")
	}
}

func priestBookings2(g *Game) int {
	_, m := priestBookings(g, 400)
	return m
}

func TestPriestGuestCleansAndDrivesGhostsOff(t *testing.T) {
	g, _ := headless(t, 2, 1, 1)
	g.startRun()
	g.unlock(Mirror)
	p := settledHuman(t, g, 0, 9)
	gh := ghostOf(g, 0, 7)
	g.ghostDirt(0, 30)
	g.becomeFather(p, false)
	if g.spiritDirt(0) == 0 {
		t.Fatal("no spiritual dirt to clean")
	}
	g.clock -= g.clock % hourFrames // the hour's tick
	g.ticked = true
	ecto := g.wallet[Ecto] // the drive-off itself pays nothing (Father M's send-over does)
	g.updatePriestGuests()
	if g.spiritDirt(0) != 0 || gh.Guests[0].PassAt == 0 || g.wallet[Ecto] != ecto {
		t.Fatalf("dirt %d, ghost passing %v, Ecto %v -> %v", g.spiritDirt(0), gh.Guests[0].PassAt != 0, ecto, g.wallet[Ecto])
	}
	until(t, g, "its ghost slot stays empty", func() bool { return !g.hasGhost(0) })
	g.ghostArrivals[0] = arrival{g.clock, g.clock}
	for range 5 * minuteFrames {
		g.step(0, 0)
	}
	if g.hasGhost(0) {
		t.Fatal("a ghost booked the Priest's Room")
	}
}

func (g *Game) hasGhost(room int) bool { return !g.vacant(room, Ghost) }

func TestPriestGuestIdentity(t *testing.T) {
	g, s, p := cardRun(t)
	g.becomeFather(p, true)
	who := p.Guests[0].Who
	if who.Name != nameFatherM || who.Job != "priest" || len(who.Lines) != 3 || who.Quirk != NoQuirk {
		t.Fatalf("identity %+v", who)
	}
	_ = s
	if txt := cardText(g); !strings.Contains(txt, nameFatherM) || !strings.Contains(txt, "priest") || !strings.Contains(txt, who.Lines[0][:10]) {
		t.Fatalf("card: %s", txt)
	}
}

// A Father has no Nerve or quirk to show: his card offers no REVEAL (menus show only what applies).
func TestPriestGuestNoReveal(t *testing.T) {
	g, _, p := cardRun(t)
	g.becomeFather(p, false)
	if slices.Contains(g.cardOpts(p, p.Guests[0]), optReveal) {
		t.Error("a Priest guest's card offers REVEAL")
	}
}

// A named guest keeps its identity: it never becomes a Father, or its visit would count for a Priest.
func TestPriestGuestNeverNamed(t *testing.T) {
	g, _ := headless(t, 2, 1, 1)
	g.startRun()
	g.grid0 = [2]int{1, 1} // as if grown: named guests come only then
	g.unlock(unlockK)
	g.unlock(unlockM)
	for range 2000 {
		g.parties = nil
		p := g.arrive(0)
		if _, ok := guestFather(p); ok && p.visit > 0 {
			t.Fatalf("a named guest's booking (visit %d) became a Father", p.visit)
		}
	}
}

// After the finale a Father guest's card also has the sun line, an answer to Magenta's studies (live test 6).
func TestFatherSunAfterFinale(t *testing.T) {
	g, _ := headless(t, 2, 1, 1)
	g.startRun()
	g.becomeFather(g.arrive(0), false)
	g.questRec.MacGuffin = true
	g.becomeFather(g.arrive(1), true)
	k, m := g.parties[0].Guests[0].Who.Lines, g.parties[1].Guests[0].Who.Lines
	if slices.Contains(k, fatherSun) || !slices.Contains(m, fatherSun) || slices.Contains(fatherLines[1], fatherSun) {
		t.Fatalf("before: %q; after: %q", k, m)
	}
}
