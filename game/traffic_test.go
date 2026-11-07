package game

import (
	"image"
	"testing"

	"ghosthotel/entities"
	"ghosthotel/world"
)

// D89, T023: at a Room doorway, a guest walking in waits for the one walking out, which goes first
// since it's nearer the choke; neither has to phase.
func TestYieldAtDoorway(t *testing.T) {
	g := mustNew(t)
	g.startRun()
	r := g.hotel.Rooms[0]
	center := func(c image.Point) image.Point {
		return c.Mul(world.TileSize).Add(image.Pt(world.TileSize/2, world.TileSize/2))
	}
	out := r.Door.Add(image.Pt(0, 3)) // the Corridor below the door
	party := func(at, to image.Point) *entities.Guest {
		guest := entities.NewGuest(at, guestCast[0])
		guest.RoomIndex = 0
		guest.SetPath(g.hotel.Path(at, to, 0))
		g.parties = append(g.parties, &Party{Guests: []*entities.Guest{&guest}, state: checkingIn, Booking: Booking{Room: 0, Checkout: 1 << 30}})
		return &guest
	}
	forGood := leaving
	leaving := party(center(r.Door), center(out))
	g.parties[0].state = forGood // out for good: once out it doesn't head back in for an activity (D168)
	coming := party(center(r.Door.Add(image.Pt(0, 3))), r.Stays[0])
	if !g.yields(coming) || g.yields(leaving) {
		t.Fatalf("yields: coming %v, leaving %v; want only the guest coming in to wait", g.yields(coming), g.yields(leaving))
	}
	for range 10 * tps {
		g.step(0, 0)
		if leaving.Phasing || coming.Phasing {
			t.Fatalf("a guest phased at the doorway: %s", where([]*entities.Guest{leaving, coming}))
		}
		if len(leaving.Path) == 0 && len(coming.Path) == 0 {
			return
		}
	}
	t.Fatalf("still walking after 10 s: %s", where([]*entities.Guest{leaving, coming}))
}
