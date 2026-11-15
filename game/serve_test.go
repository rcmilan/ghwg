package game

import "testing"

// T109, T264, D163: SPACE at an Occupied Room's door in real answers its Party's request: the Room's Mood
// rises by moodServe when answered at once, less after a wait; Fear doesn't move. With no request, or in
// ghost, nothing is served (SPACE opens the hotel view there, 08 F0). An expired ask lowers Mood by moodExpire.
func TestServe(t *testing.T) {
	g, s, p := settled(t)
	spot := center(g.hotel.Rooms[p.Room].Spot())
	g.mood[p.Room] = 0
	p.Guests[0].Fear = 7
	press(g, s, spot)
	if g.roomMood(p.Room) != 0 {
		t.Fatal("a Party that didn't ask was served")
	}
	p.asked, g.state = g.clock, StatePlay
	g.ghost = true
	press(g, s, spot)
	if g.roomMood(p.Room) != 0 || !p.asking() {
		t.Fatal("SPACE at the door in ghost served the Room")
	}
	g.ghost, p.want, g.hands, g.state = false, SupplyWater, SupplyWater, StatePlay
	press(g, s, spot)
	if m := g.roomMood(p.Room); m != moodServe || p.Guests[0].Fear != 7 || p.asking() {
		t.Fatalf("served at once: Mood %d Fear %d asking %v, want %d, 7, false", m, p.Guests[0].Fear, p.asking(), moodServe)
	}
	g.mood[p.Room] = 0
	p.asked = g.clock - askMinutes*minuteFrames/2
	g.serve(p.Room)
	if m := g.roomMood(p.Room); m != (moodServe+1)/2 { // rounded up
		t.Fatalf("served after half the wait: Mood %d, want %d", m, (moodServe+1)/2)
	}
	g.mood[p.Room] = 0
	p.asked = g.clock - askMinutes*minuteFrames
	g.serve(p.Room)
	if m := g.roomMood(p.Room); m != 0 {
		t.Fatalf("served at expiry: Mood %d, want 0", m)
	}
	g.mood[p.Room] = moodMax - 5
	p.asked = g.clock
	g.serve(p.Room)
	if m := g.roomMood(p.Room); m != moodMax {
		t.Fatalf("Mood %d, want capped at %d", m, moodMax)
	}
}

// D163, T264: an unanswered request expires after askMinutes and costs the Room moodExpire Mood; one whose
// Party leaves its Room lapses unpunished; nobody asks at night, nor again within askGap.
func TestAsk(t *testing.T) {
	g, _, p := settled(t)
	g.mood[p.Room] = 0
	p.asked = g.clock
	for range askMinutes {
		g.clock += minuteFrames
		g.updateAsks()
	}
	if p.asking() || g.roomMood(p.Room) != -moodExpire {
		t.Fatalf("expired: asking %v Mood %d, want false and %d", p.asking(), g.roomMood(p.Room), -moodExpire)
	}
	p.asked, p.state = g.clock, goingOut
	g.updateAsks()
	if p.asking() || g.roomMood(p.Room) != -moodExpire {
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
// Step 2: the Checkout pays only the tip, the stay is paid by the hour (TestRent).
func TestMoodPaysTip(t *testing.T) {
	g, _, p := settled(t)
	g.wallet[Money] = 0 // the walk in paid nothing
	plain := p.Rate
	g.mood[p.Room] = 0
	if p.Payout(g.moodPay(p.Room)) != plain {
		t.Fatalf("tip at Mood 0 %d, want one night's Rate %d", p.Payout(g.moodPay(p.Room)), plain)
	}
	p.asked = g.clock
	g.serve(p.Room)
	if p.Payout(g.moodPay(p.Room)) <= plain {
		t.Fatalf("served payout %d, want more than %d", p.Payout(g.moodPay(p.Room)), plain)
	}
	g.flee(p)
	g.parties, g.arrivals = []*Party{p}, make([]arrival, len(g.hotel.Rooms)) // nobody else is in or comes: only the fled Party could pay
	for range 60 * tps {
		g.step(0, 0)
	}
	if g.wallet[Money] != 0 {
		t.Fatalf("a fled Party paid $%v", g.wallet[Money])
	}
}

// T112: server and smart serve through the Pilot's keys, smart also calls Priests, and the
// sim's invariants hold.
func TestServeSims(t *testing.T) {
	for _, name := range []string{"server", "smart"} {
		r := RunSim(Config{Cols: 2, Rows: 1, Days: 4, Seed: 3, Strategy: Strategies[name], React: 30, KeyGap: 6})
		for _, f := range r.Failures {
			t.Errorf("%s %s: %s", name, f.Clock, f.Reason)
		}
		if r.Serves == 0 {
			t.Errorf("%s: %d serves", name, r.Serves)
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
		m := tailGeoM(x, y, balloonW, balloonH, c.hx, c.hy, balloonScale)
		if px, py := m.Apply(balloonTail-0.5, balloonTail+0.5); !c.ok(px, py) { // the tip pixel's center
			t.Errorf("Guest %s: the tail's tip at (%.1f,%.1f), box at (%d,%d) %dx%d", c.name, px, py, x, y, bw, bh)
		}
	}
}

// 08 B1 live test 4: a serve lifts the Room's Mood, which takes its normal dirt, never its spiritual: only a
// clean in ghost or a Priest does.
func TestServeLeavesSpiritualDirt(t *testing.T) {
	g, _, p := settled(t)
	g.ghostDirt(p.Room, 30)
	spirit := g.spiritDirt(p.Room)
	g.addDirtLevel(p.Room, 10)
	if spirit == 0 || g.dirtLevel(p.Room) <= spirit {
		t.Fatalf("setup: spiritual %d of %d dirt", spirit, g.dirtLevel(p.Room))
	}
	p.asked, p.want, g.hands = g.clock, SupplyWater, SupplyWater
	g.serve(p.Room)
	if g.spiritDirt(p.Room) != spirit || g.dirtLevel(p.Room) != spirit {
		t.Fatalf("after a serve: spiritual %d of %d dirt, want %d of %d", g.spiritDirt(p.Room), g.dirtLevel(p.Room), spirit, spirit)
	}
}
