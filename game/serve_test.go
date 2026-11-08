package game

import "testing"

// T109, D163: SPACE at an Occupied Room's door in real answers its Party's request: Comfort rises by
// comfortStep when answered at once, less after a wait; Fear doesn't move. With no request, or in ghost,
// nothing happens.
func TestServe(t *testing.T) {
	g, s, p := settled(t)
	spot := center(g.hotel.Rooms[p.Room].Spot())
	p.Guests[0].Fear = 7
	press(g, s, spot)
	if p.Guests[0].Comfort != 0 {
		t.Fatal("a Party that didn't ask was served")
	}
	p.asked = g.clock
	g.ghost = true
	press(g, s, spot)
	if p.Guests[0].Comfort != 0 || !p.asking() {
		t.Fatal("SPACE at the door in ghost served the Room")
	}
	g.ghost = false
	press(g, s, spot)
	if c := p.Guests[0].Comfort; c != comfortStep || p.Guests[0].Fear != 7 || p.asking() {
		t.Fatalf("served at once: Comfort %d Fear %d asking %v, want %d, 7, false", c, p.Guests[0].Fear, p.asking(), comfortStep)
	}
	p.asked = g.clock - askMinutes*minuteFrames/2
	g.serve(p.Room)
	if c := p.Guests[0].Comfort; c != comfortStep+(comfortStep+1)/2 { // rounded up
		t.Fatalf("served after half the wait: Comfort %d, want %d", c, comfortStep+(comfortStep+1)/2)
	}
	for range comfortMax {
		p.asked = g.clock
		g.serve(p.Room)
	}
	if p.Guests[0].Comfort != comfortMax {
		t.Fatalf("Comfort %d, want capped at %d", p.Guests[0].Comfort, comfortMax)
	}
	for _, c := range []struct{ spook, gain int }{{spookLimit / 2, (comfortStep + 1) / 2}, {spookLimit, 0}} {
		p.Guests[0].Comfort, p.asked, g.roomSpook[p.Room] = 0, g.clock, c.spook
		if g.serve(p.Room); p.Guests[0].Comfort != c.gain {
			t.Errorf("served at once at Room Spook %d: Comfort %d, want %d", c.spook, p.Guests[0].Comfort, c.gain)
		}
	}
}

// D163: an unanswered request expires after askMinutes and costs Comfort; one whose Party leaves its
// Room lapses unpunished; nobody asks at night, nor again within askGap.
func TestAsk(t *testing.T) {
	g, _, p := settled(t)
	p.Guests[0].Comfort = comfortStep
	p.asked = g.clock
	for range askMinutes {
		g.clock += minuteFrames
		g.updateAsks()
	}
	if p.asking() || p.Guests[0].Comfort != comfortStep-askPenalty {
		t.Fatalf("expired: asking %v Comfort %d, want false and %d", p.asking(), p.Guests[0].Comfort, comfortStep-askPenalty)
	}
	p.asked, p.state = g.clock, goingOut
	g.updateAsks()
	if p.asking() || p.Guests[0].Comfort != comfortStep-askPenalty {
		t.Fatal("a request didn't lapse cleanly as its Party left")
	}
	p.state = staying
	g.clock = (g.clock/dayFrames+1)*dayFrames + sleepHour*hourFrames - dayFrames // 23:00 today
	for range (24 - sleepHour + wakeHour) * 60 {
		g.clock += minuteFrames
		g.updateAsks()
		if p.asking() {
			t.Fatalf("asked at night, %s", clockText(g.clock))
		}
	}
	p.askEnded = g.clock
	for range askGap - 1 {
		g.clock += minuteFrames
		g.updateAsks()
		if p.asking() {
			t.Fatalf("asked again %d game minutes after the last request", (g.clock-p.askEnded)/minuteFrames)
		}
	}
	asks := 0
	for range 30 * (sleepHour - wakeHour) * 60 { // 30 awake days, the gap respected
		g.clock += minuteFrames
		if hour(g.clock) < wakeHour || hour(g.clock) >= sleepHour {
			g.clock += (24 - sleepHour + wakeHour) * hourFrames
		}
		g.updateAsks()
		if p.asking() {
			asks++
			p.asked, p.askEnded = 0, g.clock
		}
	}
	if asks < 20 || asks > 70 {
		t.Fatalf("%d requests in 30 days, want about %v a day", asks, askPerDay)
	}
}

// T110: a served Party banks more than an unserved one with the same Booking; a fled one still pays 0.
func TestComfortPays(t *testing.T) {
	g, _, p := settled(t)
	plain := p.Rate * p.Nights
	if p.Payout() != plain {
		t.Fatalf("unserved payout %d, want Rate × Nights = %d", p.Payout(), plain)
	}
	p.asked = g.clock
	g.serve(p.Room)
	if p.Payout() <= plain {
		t.Fatalf("served payout %d, want more than %d", p.Payout(), plain)
	}
	g.flee(p)
	for range 60 * tps {
		g.step(0, 0)
	}
	if g.wallet[Money] != 0 {
		t.Fatalf("a fled Party paid $%v", g.wallet[Money])
	}
}

// T112: server and smart serve through the Pilot's keys, smart also haunts and calls Priests, and the
// sim's invariants hold.
func TestServeSims(t *testing.T) {
	for _, name := range []string{"server", "smart"} {
		r := RunSim(Config{Cols: 2, Rows: 1, Days: 4, Seed: 3, Strategy: Strategies[name], React: 30, KeyGap: 6})
		for _, f := range r.Failures {
			t.Errorf("%s %s: %s", name, f.Clock, f.Reason)
		}
		if r.Serves == 0 || name == "smart" && r.Haunts == 0 {
			t.Errorf("%s: %d serves, %d haunts", name, r.Serves, r.Haunts)
		}
	}
}

// D163: the balloon's tail points at its Guest from under or over the box, never a side, slid toward them.
func TestTailTowardGuest(t *testing.T) {
	const x, y, bw, bh = 100, 100, balloonW * balloonScale, balloonH * balloonScale
	for _, c := range []struct {
		name   string
		hx, hy int
		ok     func(px, py float64) bool
	}{
		{"below", x - 20, y + bh + 50, func(px, py float64) bool { return py > y+bh && px < x+bw/2 }},
		{"above, right", x + bw + 80, y - 300, func(px, py float64) bool { return py < y && px > x+bw/2 }},
		{"beside, high", x - 300, y + 10, func(px, py float64) bool { return py < y && px < x+bw/2 }},
		{"beside, low", x + bw + 300, y + bh - 10, func(px, py float64) bool { return py > y+bh && px > x+bw/2 }},
	} {
		m := tailGeoM(x, y, balloonW, balloonH, c.hx, c.hy)
		if px, py := m.Apply(balloonTail-0.5, balloonTail+0.5); !c.ok(px, py) { // the tip pixel's center
			t.Errorf("Guest %s: the tail's tip at (%.1f,%.1f), box at (%d,%d) %dx%d", c.name, px, py, x, y, bw, bh)
		}
	}
}
