package game

import (
	"ghosthotel/entities"
	"image"
	"slices"
	"testing"
)

// T117, D165: a walk-in says "Hello!" as it passes the Concierge, and nothing far from it.
func TestHello(t *testing.T) {
	g, _ := headless(t, 2, 1, 1)
	g.startRun()
	g.player.X = -1000 // far
	p := g.walkIn()
	for range 30 {
		g.step(0, 0)
	}
	if len(p.talk) > 0 || p.greet != greetHello {
		t.Fatalf("said %v far from the Concierge", p.talk)
	}
	g.player.X, g.player.Y = p.Guests[0].X+16, p.Guests[0].Y
	g.step(0, 0)
	if len(p.talk) != 1 || !slices.Contains(helloLines, p.talk[0].text) || p.talk[0].who != 0 {
		t.Fatalf("near the Concierge: said %v, want the leader's \"Hello!\"", p.talk)
	}
}

// T117, D165: going out in the morning, a single says "Good morning!"; a pair's leader says it too, then
// an outing line to its follower, who answers.
func TestGreetPair(t *testing.T) {
	g, _, p := settled(t)
	g.clock = (g.clock/dayFrames+1)*dayFrames + 9*hourFrames
	spot := center(g.hotel.Rooms[p.Room].Spot())
	g.player.X, g.player.Y = float64(spot.X+16), float64(spot.Y)
	lead := p.Guests[0]
	lead.X, lead.Y = float64(spot.X), float64(spot.Y)
	lead.Path = []image.Point{g.hotel.Exit} // walking out
	p.Guests, p.state, p.greet = p.Guests[:1], goingOut, greetOut
	in := center(g.hotel.Rooms[p.Room].Shell())
	lead.X, lead.Y, g.ghost = float64(in.X), float64(in.Y), true
	g.player.X, g.player.Y = float64(in.X+16), float64(in.Y)
	g.updateTalk()
	if len(p.talk) > 0 || p.greet != greetOut {
		t.Fatalf("still inside its Room, in ghost, said %v", p.talk)
	}
	lead.X, lead.Y, g.ghost = float64(spot.X), float64(spot.Y), false
	g.player.X, g.player.Y = float64(spot.X+16), float64(spot.Y)
	g.updateTalk()
	if len(p.talk) != 2 || p.talk[0].text != "Good morning!" || !slices.Contains(soloOut, p.talk[1].text) {
		t.Fatalf("a single going out at 09:00 said %v", p.talk)
	}
	mate := *lead
	p.Guests, p.talk, p.greet = append(p.Guests, &mate), nil, greetOut
	g.updateTalk()
	if len(p.talk) != 3 || p.talk[0].text != "Good morning!" || p.talk[1].who != 0 || p.talk[1].at != g.tick+answerFrames ||
		!slices.ContainsFunc(outLines, func(x [2]string) bool { return x == [2]string{p.talk[1].text, p.talk[2].text} }) ||
		p.talk[2].who != 1 || p.talk[2].at != g.tick+2*answerFrames {
		t.Fatalf("a pair going out at 09:00 said %v", p.talk)
	}
	g.clock += 11 * hourFrames // 20:00
	p.talk, p.greet, p.state = nil, greetBack, checkingIn
	g.updateTalk()
	if len(p.talk) != 3 || p.talk[0].text != "Good evening!" {
		t.Fatalf("a pair back at 20:00 said %v", p.talk)
	}
}

// T118, D165: in its Room a Party talks now and then: comfy in a cozy Room, creepy in a dreadful one,
// and nothing while asking. A single never says a pair's
// line; a pair says some.
func TestRoomTalk(t *testing.T) {
	g, _, p := settled(t)
	old := eggOdds
	eggOdds = 0 // the easter egg has its own test
	t.Cleanup(func() { eggOdds = old })
	said := func(minutes int) []string {
		var lines []string
		for range minutes {
			g.clock += minuteFrames
			g.tick += minuteFrames
			g.updateTalk()
			g.updateRoomTalk()
			for _, l := range p.talk {
				if l.at == g.tick {
					lines = append(lines, l.text)
				}
			}
		}
		return lines
	}
	g.clock = (g.clock/dayFrames+1)*dayFrames + 9*hourFrames
	p.Guests = p.Guests[:1]
	lines := said(10 * 60)
	if len(lines) < 3 || slices.ContainsFunc(lines, func(l string) bool { return !slices.Contains(calmLines[0][0], l) }) {
		t.Fatalf("a single, in a cozy Room from 09:00 to 19:00, said %v; want a few of %v", lines, calmLines[0][0])
	}
	mate := *p.Guests[0]
	p.Guests = append(p.Guests, &mate)
	var pair []string
	for range 4 { // 4 awake days
		pair = append(pair, said(14*60)...)
		g.clock += 10 * hourFrames
	}
	if !slices.ContainsFunc(pair, func(l string) bool { return slices.Contains(calmLines[0][1], l) }) {
		t.Fatalf("a pair over 4 days said %v, never one of %v", pair, calmLines[0][1])
	}
	p.Guests = p.Guests[:1]
	g.clock = (g.clock/dayFrames+1)*dayFrames + 9*hourFrames
	g.mood[p.Room] = moodMin + 1
	lines = said(10 * 60)
	if len(lines) < 3 || slices.ContainsFunc(lines, func(l string) bool { return !slices.Contains(calmLines[2][0], l) }) {
		t.Fatalf("a single in a dreadful Room, said %v; want a few of %v", lines, calmLines[2][0])
	}
	p.asked = g.clock
	if lines := said(askMinutes - 1); len(lines) > 0 {
		t.Fatalf("asking, said %v", lines)
	}
}

// 2026-11-14: a line that makes sense only in a Room is never one a Guest may say or show outside; a Shadow
// seen in the hall draws a hall line; a housekeeper welcomes a human Party walking in, once.
func TestLinesByPlace(t *testing.T) {
	for _, l := range slices.Concat(slices.Concat(hallSights[:]...), helloLines, soloOut, soloBack, ghostHellos, ghostByes, ghostBacks, []string{"AAAH!", "RUN!", "Clean at last!", "Thank you!"}) {
		if roomOnly[l] {
			t.Errorf("%q is said outside the Rooms, and is a Room-only line", l)
		}
	}
	if !roomOnly["GET ME OUT!"] || !roomOnly["NO NO NO!"] || !roomOnly["Nice room!"] {
		t.Fatal("the Room's lines aren't Room-only")
	}
	g, _, p := settled(t)
	lead := p.Guests[0]
	at := g.hotel.Archway
	lead.X, lead.Y, lead.Path, lead.Anim.Facing, p.talk = float64(at.X), float64(at.Y+32), nil, entities.FaceUp, nil
	run := g.areaRun(lead.X, lead.Y)
	lo, _ := g.walls[run].ends()
	g.sighting(p, p.Guests[:1], &shade{run: run, x: lo, seen: map[*entities.Guest]int{}})
	if len(p.talk) != 1 || !slices.Contains(hallSights[0], p.talk[0].text) {
		t.Fatalf("a Shadow seen in the hall: said %v, want one of %v", p.talk, hallSights[0])
	}

	g.wallet[Money] = keeperWage
	g.hire()
	k := g.keepers[0]
	k.X, k.Y, k.Path = lead.X+16, lead.Y, nil
	p.state = checkingIn
	g.keeperTalk(k)
	if !slices.Contains(keeperWelcome, k.said.text) || !p.welcomed {
		t.Fatalf("a Party walking in past the housekeeper: it says %q", k.said.text)
	}
	k.said = line{}
	g.keeperTalk(k)
	if k.said.text != "" {
		t.Fatalf("it welcomed the same Party twice: %q", k.said.text)
	}
}

// Live test 5: the easter egg, rare, in two balloons by one speaker: a human's Riemann, a ghost's Navier-Stokes;
// never asleep.
func TestEasterEgg(t *testing.T) {
	g, _, p := settled(t)
	old := eggOdds
	eggOdds = 1
	t.Cleanup(func() { eggOdds = old })
	g.clock = (g.clock/dayFrames+1)*dayFrames + 9*hourFrames
	for _, realm := range []Realm{Human, Ghost} {
		p.Realm, p.talk = realm, nil
		for i := 0; len(p.talk) == 0; i++ {
			if i > 10*60 {
				t.Fatalf("realm %d: no line in 10 hours", realm)
			}
			g.clock += minuteFrames
			g.updateRoomTalk()
		}
		if e := eggLines[realm]; len(p.talk) != 2 || p.talk[0].text != e[0] || p.talk[1].text != e[1] || p.talk[0].who != p.talk[1].who {
			t.Fatalf("realm %d said %+v, want %v by one speaker", realm, p.talk, e)
		}
	}
	if old > .02 {
		t.Fatalf("eggOdds %v: an easter egg is rare", old)
	}
}
