package game

import (
	"slices"
	"testing"

	"ghosthotel/entities"

	"github.com/hajimehoshi/ebiten/v2"
)

// plain: a Party of generated guests, so a seed's luck never makes it a named one.
func plain(g *Game, p *Party) *Party {
	for i, guest := range p.Guests {
		guest.Who = entities.Identity{Name: []string{"Ana Costa", "Ben Berg"}[i]}
	}
	p.visit = 0
	return p
}

func lastEntry(t *testing.T, g *Game) bookEntry {
	t.Helper()
	if len(g.guestBook) == 0 {
		t.Fatal("no entry in the book")
	}
	return g.guestBook[len(g.guestBook)-1]
}

// One entry per outcome, the line from the right table.
func TestBookEntryPerOutcome(t *testing.T) {
	g, _ := headless(t, 2, 1, 3)
	g.startRun()
	g.locked = nil
	check := func(want bookOutcome, lines []string) {
		t.Helper()
		e := lastEntry(t, g)
		if e.Out != want || !slices.Contains(lines, e.Line) || e.Name == "" {
			t.Fatalf("entry %+v, want outcome %d with a line of %q", e, want, lines)
		}
	}
	p := plain(g, g.arrive(0))
	g.setDirtLevel(0, 0)
	g.checkout(p, g.hotel.Rooms[0])
	check(outCozy, notebookCozy)

	p = plain(g, g.arrive(0))
	g.setDirtLevel(0, 0)
	p.scares = 1 // it stepped out for air once
	g.checkout(p, g.hotel.Rooms[0])
	check(outDread, notebookDread)

	p = plain(g, g.arrive(0))
	g.addMood(0, -150) // the Room rotted
	g.checkout(p, g.hotel.Rooms[0])
	check(outDread, notebookDread)

	p = plain(g, g.arrive(0))
	g.flee(p)
	check(outFled, notebookFled)

	h := plain(g, g.arriveGhost(0))
	h.state, h.Guests[0].Who.Name = staying, "Agatha Fogg"
	h.Guests[0].PassAt = g.clock - passFrames
	g.updateGhost(h)
	check(outPassed, notebookPassed)

	h = plain(g, g.arriveGhost(0))
	h.state = staying
	g.sendOver(0)
	if !h.banished {
		t.Fatal("sendOver did not mark the Party banished")
	}
	g.clock += passFrames
	g.updateGhost(h)
	check(outBanished, notebookBanished)
	if len(g.guestBook) != 6 {
		t.Errorf("%d entries, want 6", len(g.guestBook))
	}
}

func TestBookKeepsLastFifty(t *testing.T) {
	g, _ := headless(t, 2, 1, 3)
	g.startRun()
	for i := range 80 {
		p := plain(g, &Party{Guests: []*entities.Guest{{}}})
		p.Guests[0].Who.Name = string(rune('A' + i%26))
		g.logStay(p, outCozy)
	}
	if len(g.guestBook) != bookMax {
		t.Fatalf("%d entries, want %d", len(g.guestBook), bookMax)
	}
}

// A human's view of the page hides the ghosts' entries; ghost shows them all, newest first.
func TestBookGhostEntriesOnlyInGhost(t *testing.T) {
	g, _ := headless(t, 2, 1, 3)
	g.guestBook = []bookEntry{{"A", "x", 1, outCozy}, {"B", "y", 1, outPassed}, {"C", "z", 1, outBanished}, {"D", "w", 1, outFled}}
	if es := g.shown(); len(es) != 2 || es[0].Name != "D" || es[1].Name != "A" {
		t.Errorf("human view %+v, want D, A", es)
	}
	g.ghost = true
	if es := g.shown(); len(es) != 4 || es[0].Name != "D" {
		t.Errorf("ghost view %+v, want all 4, newest first", es)
	}
}

// SPACE beside the book opens the page, W/S scroll, SPACE closes.
func TestBookOpensWithSpace(t *testing.T) {
	g, s := headless(t, 2, 1, 3)
	g.startRun()
	for range 20 {
		g.guestBook = append(g.guestBook, bookEntry{"A", "x", 1, outCozy})
	}
	b := g.hotel.Book
	g.player.X, g.player.Y = float64(b.Min.X+8), float64(b.Max.Y-8)
	if act, r := g.actionable(); act != actBook || r.Empty() {
		t.Fatalf("at the book SPACE does %d (%v), want the book", act, r)
	}
	s.hold(ebiten.KeySpace)
	g.Update()
	if g.state != StateBook {
		t.Fatalf("state %d after SPACE, want the book open", g.state)
	}
	s.hold(ebiten.KeyS)
	g.Update()
	if g.bookTop != 1 {
		t.Errorf("S scrolled to %d, want 1", g.bookTop)
	}
	s.hold(ebiten.KeyEscape)
	g.Update()
	if g.state != StatePlay {
		t.Errorf("ESC left state %d, want play", g.state)
	}
	if g.atTerminal() {
		t.Error("the book's column is the Terminal's")
	}
}

// About 1 in namedOdds of the Parties a Game books is a named one.
func TestNamedOdds(t *testing.T) {
	g, _ := headless(t, 2, 1, 5)
	named := 0
	for range 6000 {
		a := entities.NewGuest(g.hotel.Spawn, "Adam#1")
		p := &Party{Guests: []*entities.Guest{&a}}
		g.rollNamedHuman(p)
		if a.Who.Key != "" {
			named++
		}
	}
	if named < 800 || named > 1200 {
		t.Errorf("%d named in 6000, want about 1000", named)
	}
}

// A named guest is not in the hotel and away a game day, or it does not come.
func TestNamedAwayADay(t *testing.T) {
	g, _ := headless(t, 2, 1, 5)
	g.clock = 5 * dayFrames
	if !g.namedFree("gus") {
		t.Fatal("a guest never seen is free")
	}
	a := entities.NewGuest(g.hotel.Spawn, "Adam#1")
	a.Who.Key = "gus"
	g.parties = []*Party{{Guests: []*entities.Guest{&a}}}
	if g.namedFree("gus") {
		t.Error("in the hotel, it is free")
	}
	g.parties = nil
	g.leftAt = map[string]int{"gus": g.clock - dayFrames + 1}
	if g.namedFree("gus") {
		t.Error("gone less than a day, it is free")
	}
	g.leftAt["gus"] = g.clock - dayFrames
	if !g.namedFree("gus") {
		t.Error("gone a day, it is not free")
	}
	p := plain(g, &Party{Guests: []*entities.Guest{{}}})
	p.Guests[0].Who.Key = "gus"
	g.leftAt = nil
	g.logStay(p, outCozy)
	if g.leftAt["gus"] != g.clock {
		t.Error("an entry does not note when the named guest left")
	}
}

// A pair entry needs a Party of two and fills both; a solo one in a pair takes one.
func TestNamedPair(t *testing.T) {
	g, _ := headless(t, 2, 1, 5)
	pairs := 0
	for range 3000 {
		a, b := entities.NewGuest(g.hotel.Spawn, "Adam#1"), entities.NewGuest(g.hotel.Spawn, "Amelia#2")
		g.rollNamedHuman(&Party{Guests: []*entities.Guest{&a}})
		if a.Who.Key == "bealou" {
			t.Fatal("a pair arrived as a solo Party")
		}
		g.rollNamedHuman(&Party{Guests: []*entities.Guest{&a, &b}})
		if a.Who.Key == "bealou" {
			pairs++
			if b.Who.Key != "bealou" || a.Who.Name != "Bea Marchetti" || b.Who.Name != "Lou Marchetti" || a.Character[:6] != "Amelia" || b.Character[:4] != "Adam" {
				t.Fatalf("pair %+v %+v %s %s", a.Who, b.Who, a.Character, b.Character)
			}
		}
	}
	if pairs == 0 {
		t.Error("Bea and Lou never came in 3000 pairs")
	}
}

// Osvaldo is due only after Magda booked this Run, and trails her visits.
func TestOsvaldoFollowsMagda(t *testing.T) {
	g, _ := headless(t, 2, 1, 5)
	osvaldo := func(n int) (seen bool) {
		for range n {
			p := &Party{Guests: []*entities.Guest{{}}}
			g.rollNamedGhost(p)
			seen = seen || p.Guests[0].Who.Key == "osvaldo"
			g.leftAt = nil // all away
			g.visits["osvaldo"] = min(g.visits["osvaldo"], 0)
			if seen {
				return
			}
		}
		return
	}
	g.visits = map[string]int{}
	if g.osvaldoDue() || osvaldo(2000) {
		t.Fatal("Osvaldo came before Magda")
	}
	g.visited(&Party{}, "magda")
	if !g.osvaldoDue() || !osvaldo(2000) {
		t.Fatal("Osvaldo does not come after Magda")
	}
	g.visits["osvaldo"] = g.visits["magda"]
	if g.osvaldoDue() {
		t.Error("his visit N is due again before her visit N+1")
	}
	g.met = nil // the next Run: she has not come yet
	g.visits["osvaldo"] = 0
	if g.osvaldoDue() {
		t.Error("Osvaldo is due in a Run Magda has not booked in")
	}
}

// Visit 1..3 their own line, 4+ rotating; dread and fled win; a Skeptic's missing fled line is a scribble.
func TestNamedLines(t *testing.T) {
	gus := namedHumans[2]
	for n, want := range []string{gus.Visit[0], gus.Visit[1], gus.Visit[2], gus.Visit[0], gus.Visit[1], gus.Visit[2], gus.Visit[0]} {
		if got := namedLine("gus", n+1, outCozy); got != want {
			t.Errorf("visit %d: %q, want %q", n+1, got, want)
		}
	}
	if namedLine("gus", 5, outDread) != gus.Dread || namedLine("gus", 1, outFled) != gus.Fled {
		t.Error("dread and fled lines must win")
	}
	if namedLine("teo", 2, outFled) != "" {
		t.Error("a Skeptic has no fled line")
	}
	ned := namedGhosts[1]
	if namedLine("ned", 2, outPassed) != ned.Visit[1] || namedLine("ned", 6, outPassed) != ned.Visit[2] {
		t.Error("a ghost's visit lines")
	}
	g, _ := headless(t, 2, 1, 5)
	p := plain(g, &Party{Guests: []*entities.Guest{{}}})
	p.Guests[0].Who.Key, p.visit = "ned", 2
	if g.bookLine(p, outBanished) == "" || g.bookLine(p, outBanished) == ned.Visit[1] || g.bookLine(p, outPassed) != ned.Visit[1] {
		t.Error("a banished named ghost takes the banished table, one that passed its visit line")
	}
	p.Guests[0].Who.Key = "teo"
	if g.bookLine(p, outFled) != "" {
		t.Error("a Skeptic that walked out leaves a scribble")
	}
}

// The visits survive a prestige and a restore from the save; the notebook is the Run's.
func TestVisitsSurvivePrestigeAndRestore(t *testing.T) {
	g, _ := headless(t, 2, 1, 5)
	g.startRun()
	data, _, _ := memStore(t)
	g.persist = true
	g.visits = map[string]int{"magda": 2, "teo": 1}
	g.met = map[string]bool{"magda": true}
	g.guestBook = []bookEntry{{"Magda Feltz", "x", 2, outCozy}, {"Agatha Fogg", "y", 2, outPassed}}
	g.saveDawn()
	s := g.loadSave()
	if s == nil || s.Visits["magda"] != 2 || s.Run == nil || len(s.Run.Book) != 2 || !slices.Equal(s.Run.Met, []string{"magda"}) {
		t.Fatalf("save %s", *data)
	}
	g2, _ := headless(t, 2, 1, 9)
	g2.startRun()
	g2.persist = true
	if !g2.resume(s) || g2.visits["magda"] != 2 || g2.visits["teo"] != 1 || len(g2.guestBook) != 2 || !g2.met["magda"] || g2.guestBook[1].Out != outPassed {
		t.Fatalf("restored: visits %v, book %+v, met %v", g2.visits, g2.guestBook, g2.met)
	}
	g.startPrestige()
	if g.visits["magda"] != 2 || len(g.guestBook) != 0 || g.met != nil {
		t.Errorf("after a prestige: visits %v, book %d, met %v", g.visits, len(g.guestBook), g.met)
	}
	s = g.loadSave()
	if s == nil || s.Visits["magda"] != 2 || s.Run != nil {
		t.Errorf("the prestige's save: %+v", s)
	}
}

// Named guests book only once the hotel has grown, like the quirks they carry (T376): none in a new hotel.
func TestNamedAfterGrowth(t *testing.T) {
	g, _ := headless(t, 2, 1, 5)
	g.startRun()
	named := func() (n int) {
		for range 300 {
			g.parties, g.leftAt = nil, nil
			if p := g.arrive(0); p.Guests[0].Who.Key != "" {
				n++
			}
		}
		return
	}
	if n := named(); n != 0 {
		t.Errorf("%d named bookings before the hotel grew", n)
	}
	g.grid0 = [2]int{1, 1}
	if named() == 0 {
		t.Error("no named booking after the hotel grew")
	}
}
