package game

import (
	"image"
	"slices"
	"testing"

	"ghosthotel/entities"
	"ghosthotel/world"
)

// T141, D167: a staying Party walks into bed at sleepHour and back to its Stay Spot at wakeHour; one checking
// in at night goes to bed once in. Haunted in bed, it flees and leaves. T299 (K1): each Guest gets in and
// out by its bed's side, level with the pillow, never over the bed's foot.
func TestSleep(t *testing.T) {
	for _, beds := range []string{"Twin", "Double"} {
		g := mustNew(t)
		g.startRun()
		furnish(t, g, beds)
		p := pairWalkIn(g) // at 02:00
		r := g.hotel.Rooms[p.Room]
		at := func(guest *entities.Guest) image.Point { return image.Pt(int(guest.X), int(guest.Y)) }
		until := func(what string, done func(i int, guest *entities.Guest) bool) {
			t.Helper()
			for range 60 * tps {
				if !slices.ContainsFunc(p.Guests, func(guest *entities.Guest) bool { return !done(slices.Index(p.Guests, guest), guest) }) {
					return
				}
				g.step(0, 0)
				for _, guest := range p.Guests {
					if b, ok := g.bedAt(guest.X, guest.Y); ok && int(guest.Y) > sleepSpot(b).Y+2 { // a push may nudge it a pixel
						t.Fatalf("%s, %s: a Guest at %v crossed the foot of the bed %v", beds, what, at(guest), b)
					}
				}
			}
			t.Fatalf("%s, %s: %s", beds, what, where(p.Guests))
		}
		abed := func(i int, guest *entities.Guest) bool {
			return len(guest.Path) == 0 && at(guest) == sleepSpot(r.Lanes[i]) && guest.Anim.Facing == entities.FaceDown
		}
		until("in bed after a night check-in", abed)
		g.clock = wakeHour * hourFrames
		until("up at 07:00", func(i int, guest *entities.Guest) bool { return len(guest.Path) == 0 && at(guest) == r.Stays[i] })
		g.clock = sleepHour * hourFrames
		until("in bed at 23:00", abed)
		g.flee(p)
		for i := 0; slices.Contains(g.parties, p); i++ {
			if i > 60*tps {
				t.Fatalf("%s: fled from bed, still on the map: %s", beds, where(p.Guests))
			}
			g.step(0, 0)
		}
	}
}

// T143, T299: the Concierge lies in a bed from its side cell, never from its foot.
func TestConciergeInBed(t *testing.T) {
	for _, beds := range []string{"Single", "Twin", "Double"} {
		g := mustNew(t)
		g.startRun()
		furnish(t, g, beds)
		r := g.hotel.Rooms[0]
		for i, bed := range r.Lanes {
			foot, side := r.Stays[i], g.bedSide(r.Lanes[i])
			if g.hotel.Map.Solid(side.X/world.TileSize, side.Y/world.TileSize) || side.Y != sleepSpot(bed).Y {
				t.Fatalf("%s bed %d: its side %v is not free floor level with the pillow", beds, i, side)
			}
			g.player.X, g.player.Y = float64(foot.X), float64(foot.Y)
			for range 2 * tps {
				g.step(0, -1)
			}
			if _, ok := g.bedAt(g.player.X, g.player.Y); ok {
				t.Fatalf("%s bed %d: walking up from the Stay Spot he got onto the bed at (%.1f,%.1f)", beds, i, g.player.X, g.player.Y)
			}
			g.player.X, g.player.Y = float64(side.X), float64(side.Y)
			dx := sign(float64(sleepSpot(bed).X - side.X))
			for range 3 * tps {
				if int(g.player.X) == sleepSpot(bed).X {
					break
				}
				g.step(dx, 0)
			}
			if at, ok := g.bedAt(g.player.X, g.player.Y); !ok || at != bed || int(g.player.X) != sleepSpot(bed).X {
				t.Fatalf("%s bed %d: from its side %v he stopped at (%.1f,%.1f), want the sleep spot %v", beds, i, side, g.player.X, g.player.Y, sleepSpot(bed))
			}
		}
	}
}
