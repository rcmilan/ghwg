package game

import (
	"image"
	"slices"
	"testing"

	"ghosthotel/entities"
	"ghosthotel/world"
)

// T141, D167: a staying Party walks into bed at sleepHour, each Guest up its own lane, and back to its Stay
// Spot at wakeHour; one checking in at night goes to bed once in. Haunted in bed, it flees and leaves.
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

// T143, D167: the Concierge walks up a bed's lane from its Stay Spot to the pillow, never onto a bed from
// its side.
func TestConciergeInBed(t *testing.T) {
	g := mustNew(t)
	g.startRun()
	furnish(t, g, "Single")
	r := g.hotel.Rooms[0]
	bed, foot := r.Lanes[0], r.Stays[0]
	g.player.X, g.player.Y = float64(foot.X), float64(foot.Y)
	for range 2 * tps {
		g.step(0, -1)
	}
	if at, ok := g.bedAt(g.player.X, g.player.Y); !ok || at != bed || g.player.Y > float64(sleepSpot(bed).Y)+1 {
		t.Fatalf("walking up from the Stay Spot he stopped at (%.1f,%.1f), want the sleep spot %v", g.player.X, g.player.Y, sleepSpot(bed))
	}
	g.player.X, g.player.Y = float64(bed.Max.X+world.TileSize/2), float64(sleepSpot(bed).Y)
	for range tps {
		g.step(-1, 0)
	}
	if _, ok := g.bedAt(g.player.X, g.player.Y); ok || g.player.X-entities.FootW/2 < float64(bed.Max.X) {
		t.Fatalf("from beside the bed he stepped onto it: (%.1f,%.1f)", g.player.X, g.player.Y)
	}
}
