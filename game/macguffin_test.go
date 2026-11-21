package game

import "testing"

// The MacGuffin books the best Room once a Run, only from the first Growth, and only once summoned.
func TestMacguffinBooksBestRoomOnce(t *testing.T) {
	g, _ := headless(t, 3, 1, 1)
	g.startRun()
	g.macguffinIn()
	if g.macguffinParty() != nil {
		t.Fatal("it came before the finale")
	}
	g.questRec.MacGuffin = true
	g.macguffinIn()
	if g.macguffinParty() != nil {
		t.Fatal("it came before the first Growth")
	}
	g.grid0 = [2]int{1, 1}
	g.hotel.Rooms[1].Type.Rate = 1000 // the best
	g.macguffinIn()
	g.macguffinIn()
	n := 0
	for _, p := range g.parties {
		if isMacguffin(p) {
			n++
			if p.Room != 1 || p.Rate != macguffinPay*1000 || p.Guests[0].Who.Name != macguffinCard.Name {
				t.Errorf("booked Room %d at %d as %q", p.Room, p.Rate, p.Guests[0].Who.Name)
			}
		}
	}
	if n != 1 {
		t.Fatalf("%d MacGuffins, want 1", n)
	}
}

// It never checks out, asks, flees or is moved, whatever the Mood and Fear; it pays by the Rate alone.
func TestMacguffinStays(t *testing.T) {
	g, _ := headless(t, 2, 1, 1)
	g.startRun()
	g.questRec.MacGuffin, g.grid0 = true, [2]int{1, 1}
	p := g.macguffinCheckIn()
	g.settleIn(p)
	guest := p.Guests[0]
	g.frighten(p, guest, 1000, 1<<20)
	guest.Fear = guest.FleeAt
	g.mood[p.Room] = moodMin + 1
	if g.partyPay(p) != 1 || g.mayFlee(p, guest) || movable(p) || g.sleeps(p) {
		t.Fatal("the MacGuffin pays by Mood, flees, moves or sleeps")
	}
	for range 4 * dayFrames {
		g.step(0, 0)
	}
	if !slicesContains(g.parties, p) || p.state != staying || p.asking() || p.Fled || p.Paid {
		t.Fatalf("after 4 days: state %d, asking %v, fled %v, paid %v", p.state, p.asking(), p.Fled, p.Paid)
	}
}

func slicesContains(ps []*Party, p *Party) bool {
	for _, q := range ps {
		if q == p {
			return true
		}
	}
	return false
}

// At a Run's end its stay is written in the guest book: the one entry the next Run's notebook opens with.
func TestMacguffinSignsTheBook(t *testing.T) {
	g := dressedRun(t, 1)
	memStore(t)
	g.questRec.MacGuffin = true
	g.settleIn(g.macguffinCheckIn())
	g.resetRun()
	if len(g.guestBook) != 1 || g.guestBook[0].Name != macguffinCard.Name || g.guestBook[0].Line != macguffinCard.Book {
		t.Fatalf("the book after its Run: %v", g.guestBook)
	}
	g.resetRun() // a Run it was not in
	if len(g.guestBook) != 0 {
		t.Fatalf("the book after a Run without it: %v", g.guestBook)
	}
}
