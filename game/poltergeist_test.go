package game

import (
	"image"
	"testing"

	"ghosthotel/entities"
	"ghosthotel/world"
)

// tapSpots: a tile beside each of Room room's four walls, and its door's.
func tapSpots(g *Game, room int) map[string]image.Point {
	const ts = world.TileSize
	s := g.hotel.Rooms[room].Shell()
	cx, cy := (s.Min.X+s.Max.X)/2/ts, (s.Min.Y+s.Max.Y)/2/ts
	at := func(x, y int) image.Point { return image.Pt(x*ts+ts/2, y*ts+ts/2) }
	d := g.hotel.Rooms[room].Door
	return map[string]image.Point{
		"west": at(s.Min.X/ts-1, cy), "east": at(s.Max.X/ts, cy), "north": at(cx, s.Min.Y/ts-1), "south": at(cx+1, s.Max.Y/ts),
		"door": at(d.X, s.Max.Y/ts),
	}
}

// The tap: from a tile beside any wall (the door's too), each human inside takes tapFear, the Ecto follows
// frighten's rule, and one prop shakes; two tiles away it does nothing.
func TestPoltergeistTap(t *testing.T) {
	const ts = world.TileSize
	g, s, p := pairIn(t, 3, 2, 4)
	g.ghost = true
	p.Rate = 0
	g.player.X, g.player.Y = 0, 0
	for name, pt := range tapSpots(g, 4) {
		for _, guest := range p.Guests {
			guest.Fear = 0
		}
		g.poltTap = poltState{}
		g.tick = 100
		before := g.wallet[Ecto]
		g.player.X, g.player.Y = float64(pt.X), float64(pt.Y)
		if act, _ := g.actionable(); act != actPoltergeist {
			t.Fatalf("%s: SPACE would do %d, want the tap", name, act)
		}
		press(g, s, pt)
		for i, guest := range p.Guests {
			if guest.Fear != tapFear {
				t.Fatalf("%s: Guest %d Fear %d, want %d", name, i, guest.Fear, tapFear)
			}
		}
		want := Num(len(p.Guests)*tapFear) * Num(g.upMul(SeanceTable)*g.sightingQuirkMul(p, p.Guests[0])*g.sightingPay(4)) * Num(g.bonus)
		if got := g.wallet[Ecto] - before; got <= 0 || got != want {
			t.Fatalf("%s: earned %v Ecto, want %v", name, got, want)
		}
		c := g.poltTap.cell
		if g.poltTap.at == 0 || !image.Pt(c.X*ts, c.Y*ts).In(g.hotel.Rooms[4].Shell()) {
			t.Fatalf("%s: shaken prop %v not in the Room", name, c)
		}
		shaking := 0
		for tick := 100; tick < 100+tapShakeFrames; tick++ {
			g.tick = tick
			if g.tapShake(c, world.LayerProps) != 0 {
				shaking++
			}
			if g.tapShake(c.Add(image.Pt(1, 0)), world.LayerProps) != 0 {
				t.Fatalf("%s: a second prop shakes", name)
			}
		}
		if shaking == 0 {
			t.Fatalf("%s: the prop never shakes", name)
		}
		g.tick = 100 + tapShakeFrames
		if g.tapShake(c, world.LayerProps) != 0 {
			t.Fatalf("%s: the prop still shakes after tapShakeFrames", name)
		}
	}
	// two tiles from the west wall
	g.tick = 500
	for _, guest := range p.Guests {
		guest.Fear = 0
	}
	sh := g.hotel.Rooms[4].Shell()
	far := image.Pt(sh.Min.X-ts-ts/2, (sh.Min.Y+sh.Max.Y)/2)
	if beside(g.hotel, far) == 4 {
		t.Fatal("two tiles away is beside")
	}
	g.player.X, g.player.Y = float64(far.X), float64(far.Y)
	if g.poltTapNow() || p.Guests[0].Fear != 0 {
		t.Fatalf("tap from two tiles away worked: Fear %d", p.Guests[0].Fear)
	}
}

// Refused: in real, with no human inside, and with only ghosts inside; it never frightens a ghost Guest.
func TestPoltergeistRefused(t *testing.T) {
	g, s, p := pairIn(t, 3, 2, 4)
	p.Rate = 0
	pt := tapSpots(g, 4)["west"]
	press(g, s, pt) // real
	if p.Guests[0].Fear != 0 || g.poltTap.at != 0 {
		t.Fatalf("tapped in real: Fear %d", p.Guests[0].Fear)
	}
	g.ghost = true
	for _, guest := range p.Guests { // nobody inside: out for the day
		guest.X, guest.Y = 0, 0
	}
	g.player.X, g.player.Y = float64(pt.X), float64(pt.Y)
	if g.poltTapNow() || g.wallet[Ecto] != 0 {
		t.Fatal("tapped a Room with no human inside")
	}
	r := g.hotel.Rooms[4]
	h, gh := entities.NewGuest(r.Stays[0], "Adam"), entities.NewGuest(r.Stays[0], "Wisp")
	g.parties = []*Party{{Guests: []*entities.Guest{&gh}, state: staying, Booking: Booking{Realm: Ghost, Room: 4, Nights: 1, Checkout: 99 * dayFrames}}}
	if g.poltTapNow() || gh.Fear != 0 {
		t.Fatalf("a ghost alone inside: its Fear %d", gh.Fear)
	}
	hp := &Party{Guests: []*entities.Guest{&h}, state: staying, Booking: Booking{Room: 4, Nights: 1, Checkout: 99 * dayFrames}}
	g.parties = append(g.parties, hp)
	if !g.poltTapNow() || h.Fear != tapFear || gh.Fear != 0 {
		t.Fatalf("human with a ghost in the Room: human Fear %d, ghost Fear %d; want %d and 0", h.Fear, gh.Fear, tapFear)
	}
}

// The cap is the Guest's FleeAt, as a haunt's: enough taps and the Party flees; each tap earns only the Fear it adds.
func TestPoltergeistCap(t *testing.T) {
	g, _, p := pairIn(t, 3, 2, 4)
	g.ghost = true
	pt := tapSpots(g, 4)["west"]
	g.player.X, g.player.Y = float64(pt.X), float64(pt.Y)
	taps := 0
	for g.poltTapNow() {
		taps++
	}
	if want := (p.Guests[0].FleeAt + tapFear - 1) / tapFear; taps != want || p.Guests[0].Fear != p.Guests[0].FleeAt {
		t.Fatalf("%d taps to FleeAt, Fear %d; want %d taps", taps, p.Guests[0].Fear, want)
	}
}
