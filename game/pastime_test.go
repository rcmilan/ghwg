package game

import (
	"image"
	"slices"
	"testing"

	"ghosthotel/entities"
	"ghosthotel/world"
)

// T151, D168: from 07:00 to 23:00 a staying pair uses the Room's Spots facing their props, never both on
// one; rests on its beds; idles at its Stay Spots glancing around; and chats facing each other.
func TestPastime(t *testing.T) {
	g := mustNew(t)
	g.startRun()
	furnish(t, g, "Twin")
	p := pairWalkIn(g)
	p.Routine, p.Checkout = homebody, 10*dayFrames
	r := g.hotel.Rooms[p.Room]
	for len(p.Guests[0].Path)+len(p.Guests[1].Path) > 0 || p.state != staying {
		g.step(0, 0)
	}
	g.clock = wakeHour * hourFrames
	at := func(guest *entities.Guest) image.Point { return image.Pt(int(guest.X), int(guest.Y)) }
	used, turns, last, idled := map[string]bool{}, 0, [2]entities.Facing{}, [2]bool{}
	rested, chatted, still := false, false, [2]bool{}
	for g.clock < sleepHour*hourFrames {
		g.step(0, 0)
		a, b := p.Guests[0], p.Guests[1]
		if p.acts[0].at == p.acts[1].at {
			t.Fatalf("both Guests headed for %v", p.acts[0].at)
		}
		chatted = chatted || len(a.Path)+len(b.Path) == 0 && !p.acts[0].idle && !p.acts[1].idle && at(a) == r.Stays[0] && at(b) == r.Stays[1] &&
			a.Anim.Facing == entities.FaceRight && b.Anim.Facing == entities.FaceLeft
		for i, guest := range p.Guests {
			if was := still[i]; !was || len(guest.Path) > 0 { // turned to its prop the frame after it arrives
				still[i] = len(guest.Path) == 0
				continue
			}
			if s := slices.IndexFunc(r.Spots, func(s world.Spot) bool { return s.At == at(guest) }); s >= 0 {
				d := r.Spots[s].Prop.Sub(r.Spots[s].At)
				if guest.Anim.Facing != entities.Toward(float64(d.X), float64(d.Y)) {
					t.Fatalf("Guest %d at the %s spot faces %v", i, r.Spots[s].Verb, guest.Anim.Facing)
				}
				used[r.Spots[s].Verb+string(rune('0'+s))] = true
			}
			rested = rested || at(guest) == sleepSpot(r.Lanes[i])
			idle := at(guest) == r.Stays[i] && p.acts[i].idle
			if idle && idled[i] && guest.Anim.Facing != last[i] {
				turns++
			}
			last[i], idled[i] = guest.Anim.Facing, idle
		}
	}
	if len(used) != len(r.Spots) || !rested || turns < 20 || !chatted {
		t.Fatalf("over a day: spots used %v of %d, rested %v, glances while idle %d, chatted %v", used, len(r.Spots), rested, turns, chatted)
	}
	for range 60 * tps { // and to bed at 23:00
		g.step(0, 0)
	}
	for i, guest := range p.Guests {
		if at(guest) != sleepSpot(r.Lanes[i]) {
			t.Fatalf("Guest %d at %v after 23:00, want in bed", i, at(guest))
		}
	}
}
