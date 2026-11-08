package game

import (
	"image"
	"testing"

	"ghosthotel/entities"

	"github.com/hajimehoshi/ebiten/v2"
)

// settled: a fresh Run with one Homebody Party walked into its Room.
func settled(t *testing.T) (*Game, *script, *Party) {
	t.Helper()
	g, s := headless(t, 2, 1, 1)
	g.startRun()
	p := g.walkIn()
	p.Routine = homebody
	for i := 0; p.state != staying; i++ {
		if i > 60*tps {
			t.Fatal("the Party never reached its Room")
		}
		g.step(0, 0)
	}
	return g, s, p
}

func center(r image.Rectangle) image.Point { return r.Min.Add(r.Max).Div(2) }

// press puts the Concierge at pt and taps SPACE through Update.
func press(g *Game, s *script, pt image.Point) {
	g.player.X, g.player.Y = float64(pt.X), float64(pt.Y)
	s.hold()
	g.Update()
	s.hold(ebiten.KeySpace)
	g.Update()
}

// T097 (D150): SPACE at an Occupied Room's door spot haunts it in the ghost dimension only, for
// hauntMinutes; again restarts the timer.
func TestHaunt(t *testing.T) {
	g, s, p := settled(t)
	spot := center(g.hotel.Rooms[p.Room].Spot())
	press(g, s, spot)
	if g.haunting(p.Room) {
		t.Fatal("SPACE at the door haunted the Room in the real dimension")
	}
	g.ghost = true
	press(g, s, spot)
	if !g.haunting(p.Room) || g.state != StatePlay {
		t.Fatal("SPACE at the door in ghost didn't haunt the Room")
	}
	g.clock += hauntMinutes*minuteFrames - 2*minuteFrames
	press(g, s, spot) // restarts
	if g.haunted[p.Room] != g.clock+hauntMinutes*minuteFrames {
		t.Fatalf("haunting again ends at %d, want %d", g.haunted[p.Room], g.clock+hauntMinutes*minuteFrames)
	}
	g.clock = g.haunted[p.Room]
	if g.haunting(p.Room) {
		t.Fatal("the haunt outlasted hauntMinutes")
	}
	other := 1 - p.Room     // Vacant: nothing to haunt
	g.parties = []*Party{p} // Day 1 lets a second Party in (D163)
	press(g, s, center(g.hotel.Rooms[other].Spot()))
	if g.haunting(other) {
		t.Fatal("a Vacant Room was haunted")
	}
}

// T098 (D146): Fear rises while haunted and inside; at fearLimit the Party flees unpaid, and its Room is
// Vacant at once for the next Walk-in.
func TestFlee(t *testing.T) {
	g, _, p := settled(t)
	room := p.Room
	for i := 0; !p.Fled; i++ {
		if i > 2*fearLimit/fearPerMinute*minuteFrames {
			t.Fatalf("no flight: Fear %d", p.Guests[0].Fear)
		}
		if !g.haunting(room) {
			g.haunt(room)
		}
		g.step(0, 0)
	}
	if !g.vacant(room) || p.state != leaving {
		t.Fatalf("fled: vacant %v state %d, want Vacant and leaving", g.vacant(room), p.state)
	}
	for i := 0; len(g.parties) > 0 && g.parties[0] == p; i++ {
		if i > 60*tps {
			t.Fatal("the fled Party never left")
		}
		g.step(0, 0)
	}
	if g.wallet[Money] != 0 || p.Paid {
		t.Fatalf("a fled Party paid: money %v", g.wallet[Money])
	}
}

// T098: a Tourist out of the hotel can't be scared.
func TestOutNotScared(t *testing.T) {
	g, _, p := settled(t)
	p.state = out
	g.haunt(p.Room)
	g.scare()
	if p.Guests[0].Fear != 0 {
		t.Fatalf("an outing Party gained Fear %d", p.Guests[0].Fear)
	}
}

// T099 (D148): every point of Fear a haunt adds is a point of Spook.
func TestSpook(t *testing.T) {
	g, _ := headless(t, 2, 1, 1)
	g.startRun()
	r := g.hotel.Rooms[0]
	a, b := entities.NewGuest(r.Stays[0], "Adam"), entities.NewGuest(r.Stays[len(r.Stays)-1], "Alex")
	g.parties = []*Party{{Guests: []*entities.Guest{&a, &b}, state: staying, Booking: Booking{Room: 0, Checkout: 99 * dayFrames}}}
	g.haunt(0)
	g.scare()
	if g.wallet[Spook] != 2*fearPerMinute || a.Fear != fearPerMinute {
		t.Fatalf("one haunt-minute on a pair: Spook %v, Fear %d; want %d and %d", g.wallet[Spook], a.Fear, 2*fearPerMinute, fearPerMinute)
	}
}

// T101: the haunter haunts by keys alone through the Pilot, and the sim's invariants hold: Spook only
// from a haunt, no fled Party pays.
func TestHaunterSim(t *testing.T) {
	r := RunSim(Config{Cols: 2, Rows: 1, Days: 2, Seed: 2, Strategy: Strategies["haunter"], React: 30, KeyGap: 6})
	for _, f := range r.Failures {
		t.Errorf("%s: %s", f.Clock, f.Reason)
	}
	if r.Haunts == 0 || r.Spook == 0 || r.Fled == 0 {
		t.Fatalf("haunter: %d haunts, %d Spook, %d fled; want all > 0", r.Haunts, r.Spook, r.Fled)
	}
}
