package game

import (
	"image"
	"slices"
	"testing"

	"ghosthotel/entities"
	"ghosthotel/world"

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

// pairIn: a fresh Run on a cols×rows grid with a staying pair at the Stay Spots of Room room.
func pairIn(t *testing.T, cols, rows, room int) (*Game, *script, *Party) {
	t.Helper()
	g, s := headless(t, cols, rows, 1)
	g.startRun()
	r := g.hotel.Rooms[room]
	a, b := entities.NewGuest(r.Stays[0], "Adam"), entities.NewGuest(r.Stays[len(r.Stays)-1], "Alex")
	p := &Party{Guests: []*entities.Guest{&a, &b}, state: staying, Booking: Booking{Room: room, Nights: 1, Rate: 100, Checkout: 99 * dayFrames}}
	g.parties = []*Party{p}
	return g, s, p
}

// T249: the Hex is gone: no store row, no Unlock, and SPACE in ghost beside a Room with Guests changes nothing.
func TestNoHex(t *testing.T) {
	for _, it := range shop {
		if it.ID == "hex" || it.Name == "Hex" {
			t.Fatalf("the store still sells %q", it.Name)
		}
	}
	const ts = world.TileSize
	g, s, p := pairIn(t, 3, 2, 4)
	o := g.hotel.Rooms[4].Origin
	g.ghost = true
	moods := slices.Clone(g.mood)
	press(g, s, image.Pt((o.X-1)*ts+ts/2, (o.Y+5)*ts+ts/2)) // beside the west wall
	if g.wallet != [2]Num{} || len(g.floaters) > 0 || p.Guests[0].Fear != 0 || !slices.Equal(g.mood, moods) || g.taps[4] != [taskCount]Num{} {
		t.Fatalf("SPACE beside an occupied Room did something: wallet %v, floaters %d, Fear %d, taps %v", g.wallet, len(g.floaters), p.Guests[0].Fear, g.taps[4])
	}
}

// T191: at rotMood a Room rots: each Guest inside loses rotComfort a game minute, so the Party pays
// less, and its floor shows full; at moodMin it closes as before.
func TestRot(t *testing.T) {
	if rotMood <= moodMin {
		t.Fatalf("rotMood %d must be above moodMin %d", rotMood, moodMin)
	}
	g, _, p := pairIn(t, 2, 1, 0)
	pay := p.Payout()
	g.mood[0] = rotMood + 1
	g.scare()
	if p.Guests[0].Comfort != 0 || g.roomShift(0) >= 1 {
		t.Fatalf("above rotMood: Comfort %d, floor %v; want 0 and under 1", p.Guests[0].Comfort, g.roomShift(0))
	}
	g.mood[0] = rotMood
	g.scare()
	g.scare()
	if p.Guests[0].Comfort != -2*rotComfort || p.Payout() >= pay || g.roomShift(0) != 1 || g.closed(0) {
		t.Fatalf("2 rotting minutes: Comfort %d, pay %d (was %d), floor %v, closed %v", p.Guests[0].Comfort, p.Payout(), pay, g.roomShift(0), g.closed(0))
	}
	for range 10 * comfortMax {
		g.scare()
	}
	if p.Guests[0].Comfort != -comfortMax || p.Payout() <= 0 {
		t.Fatalf("long rot: Comfort %d, pay %d; want floored at %d, still paying", p.Guests[0].Comfort, p.Payout(), -comfortMax)
	}
	if g.mood[0] = moodMin; !g.closed(0) {
		t.Fatal("not Closed at moodMin")
	}
}

// T098 (D146): frights raise Fear; at fearLimit the Party steps out to its door for coolMinutes and comes
// back with its Fear cut (Step 2); at fearLimit again it flees unpaid, and its Room is Vacant at once.
func TestFlee(t *testing.T) {
	g, _, p := settled(t)
	g.ghost = true
	room := p.Room
	shell := g.hotel.Rooms[room].Shell()
	out := 0
	for i := 0; !p.Fled; i++ {
		if i > 6*fearLimit*minuteFrames {
			t.Fatalf("no flight: Fear %d, state %d", p.Guests[0].Fear, p.state)
		}
		if i%(2*minuteFrames) == 0 && p.state == staying {
			g.frighten(p, p.Guests[0], 1, fearLimit)
		}
		g.step(0, 0)
		if p.state == cooling && !image.Pt(int(p.Guests[0].X), int(p.Guests[0].Y)).In(shell) {
			out++
		}
	}
	if p.scares != 2 || out < coolMinutes*minuteFrames/3 || g.wallet[Money] == 0 {
		t.Fatalf("fled after %d limits, %d frames out cooling, $%v paid; want 2, a spell at the door, some rent", p.scares, out, g.wallet[Money])
	}
	if !g.vacant(room, Human) || p.state != leaving {
		t.Fatalf("fled: vacant %v state %d, want Vacant and leaving", g.vacant(room, Human), p.state)
	}
	rent := g.wallet[Money]
	for i := 0; len(g.parties) > 0 && g.parties[0] == p; i++ {
		if i > 60*tps {
			t.Fatal("the fled Party never left")
		}
		g.step(0, 0)
	}
	if g.wallet[Money] != rent || p.Paid {
		t.Fatalf("a fled Party paid past its rent: money %v, rent %v", g.wallet[Money], rent)
	}
}
