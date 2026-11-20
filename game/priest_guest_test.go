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
	if k, m := priestBookings(g, 400); k == 0 || m != 0 {
		t.Fatalf("K unlocked only: %d K, %d M", k, m)
	}
	g.unlock(unlockM)
	if k, m := priestBookings(g, 400); k == 0 || m == 0 {
		t.Fatalf("both unlocked: %d K, %d M", k, m)
	}
}

func TestPriestGuestNotWhileRetained(t *testing.T) {
	g, _ := headless(t, 2, 1, 1)
	g.startRun()
	g.unlock(unlockK)
	g.retainK()
	if k, _ := priestBookings(g, 400); k != 0 {
		t.Fatalf("a retained Father K booked %d times", k)
	}
}

func TestOneOfEachFatherBothWays(t *testing.T) {
	g, _ := headless(t, 2, 1, 1)
	g.startRun()
	g.unlock(unlockK)
	g.wallet[Ecto] = 1000
	p := g.arrive(0)
	g.becomeFather(p, false)
	if !g.fatherIn(false) || g.noFather(1, false) == "" || g.callPriest(1, false) == "" {
		t.Fatal("Father K can be called while he is a guest")
	}
	if g.noFather(1, true) != "" {
		t.Fatal("Father M is blocked by Father K")
	}
	g.parties = nil
	g.sendPriest(1, false)
	if priestBookings2(g) != 0 {
		t.Fatal("a called Father K booked as a guest")
	}
}

func priestBookings2(g *Game) int {
	k, _ := priestBookings(g, 400)
	return k
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
	if who.Name != "Father M" || who.Job != "priest" || who.Card == "" || who.Quirk != NoQuirk {
		t.Fatalf("identity %+v", who)
	}
	_ = s
	if txt := cardText(g); !strings.Contains(txt, "Father M") || !strings.Contains(txt, "priest") || !strings.Contains(txt, who.Card[:10]) {
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
