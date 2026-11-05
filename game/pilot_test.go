package game

import (
	"fmt"
	"testing"
)

type nop struct{}

func (nop) Act(View, *Pilot) {}

// run steps the autopiloted Game until done or the frame budget runs out.
func run(t *testing.T, g *Game, frames int, done func() bool) {
	t.Helper()
	for range frames {
		if g.Update(); done() {
			return
		}
	}
	t.Fatalf("not done after %d frames, at %s", frames, clockText(g.clock))
}

// T053: the Pilot books by keys alone, at the KeyGap pace, and a refused pick fails with the Terminal shut.
func TestPilot(t *testing.T) {
	for _, grid := range [][2]int{{2, 1}, {3, 3}} {
		t.Run(fmt.Sprintf("%dx%d", grid[0], grid[1]), func(t *testing.T) {
			g, _ := headless(t, grid[0], grid[1], 7)
			a := g.autopilot(nop{}, 30, 6)
			p := a.pilot
			front := func() bool { return g.state == StatePlay && g.view().Front != nil }
			run(t, g, 2*dayFrames, front)
			room := firstFit(g.view())
			p.GoTo(g.hotel.Archway) // walk away first, so OpenTerminal has to walk back
			p.OpenTerminal()
			p.Book(room)
			last, minGap := p.lastTap, 1<<30
			run(t, g, 60*tps, func() bool {
				if p.lastTap != last {
					minGap, last = min(minGap, p.lastTap-last), p.lastTap
				}
				return p.Idle()
			})
			if v := g.view(); v.Rooms[room].Vacant || v.Terminal.Open || p.Refused != 0 {
				t.Fatalf("after Book(%d): vacant %v, Terminal open %v, refused %d; want it booked and the Terminal shut", room, v.Rooms[room].Vacant, v.Terminal.Open, p.Refused)
			}
			if minGap < 6 {
				t.Fatalf("two taps %d frames apart, want KeyGap 6 at least", minGap)
			}

			// nobody at the counter now: the pick is refused
			p.OpenTerminal()
			p.Book(room)
			run(t, g, 60*tps, p.Idle)
			if v := g.view(); v.Terminal.Open || p.Refused != 1 {
				t.Fatalf("Book with nobody at the counter: Terminal open %v, refused %d; want it shut and 1 refused", v.Terminal.Open, p.Refused)
			}
		})
	}
}

// T054: firstfit books Parties by keys; idle never does.
func TestStrategy(t *testing.T) {
	for _, grid := range [][2]int{{2, 1}, {3, 3}} {
		for name, s := range map[string]Strategy{"firstfit": stand{atTerminal, firstFit}, "idle": Strategies["idle"]} {
			t.Run(fmt.Sprintf("%dx%d/%s", grid[0], grid[1], name), func(t *testing.T) {
				g, _ := headless(t, grid[0], grid[1], 3)
				g.autopilot(s, 30, 6)
				for g.clock < runStart+dayFrames || g.state == StateTitle {
					g.Update()
				}
				booked := 0
				for _, p := range g.parties {
					if p.Room >= 0 {
						booked++
					}
				}
				if (name == "idle") != (booked == 0) {
					t.Fatalf("%d Bookings in a day", booked)
				}
			})
		}
	}
}

// T059: bestrate takes the Room paying the front Party most per night, the Wish halving a wrong layout.
func TestBestRate(t *testing.T) {
	rooms := []RoomView{
		{Beds: "Single", Vacant: true, Capacity: 1, Rate: 40},
		{Beds: "Double", Vacant: true, Capacity: 2, Rate: 60},
		{Beds: "Twin", Vacant: true, Capacity: 2, Rate: 50},
		{Beds: "Twin", Vacant: false, Capacity: 2, Rate: 90},
	}
	for _, c := range []struct {
		front Front
		want  int
	}{
		{Front{Size: 1}, 1},
		{Front{Size: 2, Wish: "Double"}, 1},
		{Front{Size: 2, Wish: "Twin"}, 2}, // 50 beats 60/2
	} {
		if got := bestRate(View{Rooms: rooms, Front: &c.front}); got != c.want {
			t.Errorf("front %+v: Room %d, want %d", c.front, got, c.want)
		}
	}
}
