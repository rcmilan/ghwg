package game

import (
	"slices"
	"testing"

	"shift/world"

	"github.com/hajimehoshi/ebiten/v2"
)

// T069: SPACE at a Room's Door Panel flips it between manual and auto, any time; the Spin keeps it; a new
// Run starts all manual.
func TestDoorPanel(t *testing.T) {
	g, s := headless(t, 3, 3, 1)
	s.hold(ebiten.KeySpace)
	g.Update()
	spot := g.hotel.Rooms[0].PanelSpot()
	g.player.X, g.player.Y = float64(spot.X), float64(spot.Y)
	press := func() {
		s.hold()
		g.Update()
		s.hold(ebiten.KeySpace)
		g.Update()
	}
	press()
	if !g.autoRoom[0] || g.state != StatePlay {
		t.Fatalf("SPACE at Room 0's panel: auto %v, state %d; want auto, still playing", g.autoRoom[0], g.state)
	}
	for i := range g.autoRoom[1:] {
		if g.autoRoom[i+1] {
			t.Fatalf("Room %d flipped too", i+1)
		}
	}
	g.midnight()
	for s.hold(); g.spinning(); g.Update() {
	}
	if !g.autoRoom[0] {
		t.Fatal("the Spin reset Room 0 to manual")
	}
	press()
	if g.autoRoom[0] {
		t.Fatal("a second SPACE left Room 0 on auto")
	}
	press()
	g.startRun()
	if g.autoRoom[0] {
		t.Fatal("a new Run kept Room 0 on auto")
	}
	g.player.X, g.player.Y = float64(g.hotel.Archway.X), float64(g.hotel.Archway.Y)
	press()
	for i, a := range g.autoRoom {
		if a {
			t.Fatalf("SPACE away from every panel flipped Room %d", i)
		}
	}
}

// T072: the Pilot flips a Door Panel by keys alone, and leaves one already set as asked.
func TestPilotSetAuto(t *testing.T) {
	for _, grid := range [][2]int{{2, 1}, {3, 3}} {
		g, _ := headless(t, grid[0], grid[1], 7)
		p := g.autopilot(nop{}, 30, 6).pilot
		run(t, g, 10, func() bool { return g.state == StatePlay })
		room := len(g.hotel.Rooms) - 1
		p.SetAuto(room, true)
		run(t, g, 60*tps, p.Idle)
		if !g.autoRoom[room] || !g.view().Rooms[room].Auto {
			t.Fatalf("%v: after SetAuto(%d, true) the Room is manual", grid, room)
		}
		last := p.lastTap
		p.SetAuto(room, true)
		run(t, g, 60*tps, p.Idle)
		if p.lastTap != last || !g.autoRoom[room] {
			t.Fatalf("%v: SetAuto on a Room already auto pressed a key", grid)
		}
		p.SetAuto(room, false)
		run(t, g, 60*tps, p.Idle)
		if g.autoRoom[room] {
			t.Fatalf("%v: SetAuto(%d, false) left it auto", grid, room)
		}
	}
}

// autoHotel: a Run on 3×3 whose Rooms are all of the named bed layouts, in Room order, all manual,
// with Parties of the given sizes walked to their lanes.
func autoHotel(t *testing.T, beds []string, sizes ...int) (*Game, []*Party) {
	t.Helper()
	g, _ := headless(t, 3, 3, 1)
	g.startRun()
	next := make([]world.RoomType, len(g.hotel.Rooms))
	for i := range next {
		k := slices.IndexFunc(g.types, func(rt world.RoomType) bool { return rt.Beds == beds[i%len(beds)] && rt.Name == "Standard" })
		next[i] = g.types[k]
	}
	g.spin(next, nil, true)
	var ps []*Party
	for _, n := range sizes {
		for {
			p := g.walkIn()
			if len(p.Guests) == n {
				ps = append(ps, p)
				break
			}
			g.queue, g.parties = g.queue[:len(g.queue)-1], g.parties[:len(g.parties)-1]
		}
	}
	for i := 0; slices.ContainsFunc(ps, func(p *Party) bool { return !p.standing() }); i++ {
		if i > 60*tps {
			t.Fatal("the Parties never reached their lanes")
		}
		g.step(0, 0)
	}
	return g, ps
}

// T074: Auto Rooms book the first counter Party they fit, best payout first, for 85%; never an amber
// Room, never while the Terminal is open.
func TestAutoBook(t *testing.T) {
	t.Run("skips a front Party nothing fits", func(t *testing.T) {
		g, ps := autoHotel(t, []string{"Double", "Single"}, 2, 1)
		g.autoRoom[1] = true // a Single
		g.step(0, 0)
		if pair, solo := ps[0], ps[1]; pair.Room >= 0 || solo.Room != 1 || !solo.Auto {
			t.Fatalf("pair in %d, solo in %d (auto %v); want the pair queued, the solo auto-booked into Room 1", pair.Room, solo.Room, solo.Auto)
		}
	})
	t.Run("wrong layout at half", func(t *testing.T) {
		g, ps := autoHotel(t, []string{"Twin"}, 2)
		ps[0].Wish = "Double"
		g.autoRoom[4] = true
		g.step(0, 0)
		if p := ps[0]; p.Room != 4 || p.Rate != g.hotel.Rooms[4].Type.Rate/2 {
			t.Fatalf("pair wishing Double: Room %d at $%d; want the green Twin 4 at half its $%d", p.Room, p.Rate, g.hotel.Rooms[4].Type.Rate)
		}
	})
	t.Run("best payout, then lowest number", func(t *testing.T) {
		g, ps := autoHotel(t, []string{"Twin", "Double", "Double"}, 2)
		ps[0].Wish = "Double"
		g.autoRoom[0], g.autoRoom[1], g.autoRoom[8] = true, true, true // a Twin (half), 302's Double, 103's Double
		g.step(0, 0)
		if ps[0].Room != 8 {
			t.Fatalf("booked Room %s, want 103: a full-Rate Double, the lowest number of those", g.hotel.Rooms[ps[0].Room].Number)
		}
	})
	t.Run("amber Rooms and an open Terminal", func(t *testing.T) {
		g, ps := autoHotel(t, []string{"Single", "Twin", "Double"}, 1)
		g.step(0, 0)
		if ps[0].Room >= 0 {
			t.Fatal("booked with every Room amber")
		}
		g.autoRoom[0] = true
		g.openTerminal()
		g.Update()
		if ps[0].Room >= 0 {
			t.Fatal("auto-booked while the Terminal was open")
		}
		g.state = StatePlay
		g.Update()
		if ps[0].Room != 0 {
			t.Fatal("not booked once the Terminal shut")
		}
	})
	if p := (Booking{Rate: 64, Nights: 3, Auto: true}).Payout(); p != 163 {
		t.Errorf("auto $64 × 3 nights pays %d, want 163 (85%% rounded down)", p)
	}
}

// T075, T076: autoall books only through Auto Rooms, pays 85%, and breaks no invariant.
func TestSimAutoAll(t *testing.T) {
	r := RunSim(Config{Cols: 3, Rows: 3, Days: 3, Seed: 3, Strategy: Strategies["autoall"], React: 30, KeyGap: 6})
	if len(r.Failures) > 0 || r.Auto == 0 || r.Auto != r.Bookings || r.Paid == 0 || r.Fees == 0 || sum(r.MoneyByDay) == 0 {
		t.Fatalf("%+v; want only auto Bookings, some paid with fees, no failures", r)
	}
}
