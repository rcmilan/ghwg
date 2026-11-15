package game

import (
	"image"
	"io/fs"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"ghosthotel/entities"
	"ghosthotel/world"
)

// T247: the currency is Ectoplasm and the old name is gone from every .go file in the repo.
func TestNoOldCurrencyName(t *testing.T) {
	if runtime.GOOS == "js" {
		t.Skip("reads the source files, which Node on Windows cannot list")
	}
	old := "spo" + "ok"
	err := filepath.WalkDir("..", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || filepath.Ext(path) != ".go" {
			return err
		}
		b, err := os.ReadFile(path)
		if err == nil && strings.Contains(strings.ToLower(string(b)), old) {
			t.Errorf("%s still says %q", path, old)
		}
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	if Ecto.String() != "Ectoplasm" {
		t.Fatalf("Ecto displays as %q", Ecto.String())
	}
}

// T248, T266: dirt is Mood's face: none from moodSpots up, then one spot unit per ~3.75 points. For every dirt
// value 0..dirtMax the table of what a Room does is 06's: spot count, Closed, the Priest's halve, and the Ecto
// harvested by cleaning; a clean takes the Mood to moodClean.
func TestMoodMatchesTrunk(t *testing.T) {
	g, _ := headless(t, 2, 1, 1)
	g.startRun()
	oldSpots := func(s int) (n int) {
		for total := 0; total < s; n++ {
			total += 5 * max(1, n+2-4) // trunk: spot unit × max(1, spots+2-bigFrom)
		}
		return n
	}
	if g.roomMood(0) != moodStart || g.dirtLevel(0) != 0 || moodOf(0) != moodSpots {
		t.Fatalf("a new Run: Mood %d, dirt %d; no dirt at Mood %d", g.roomMood(0), g.dirtLevel(0), moodSpots)
	}
	for s := 0; s <= dirtMax; s++ {
		m := moodOf(s)
		if d := float64(m) - max(moodMin, moodSpots-float64(s)*4/3); d > 0.5 || d < -0.5 {
			t.Fatalf("dirt %d: Mood %d, want %d - %d×4/3 = %.1f", s, m, moodSpots, s, moodSpots-float64(s)*4/3)
		}
		g.mood[0], g.dirt, g.dirtN = m, nil, nil
		if g.dirtLevel(0) != s {
			t.Fatalf("dirt %d round-trips to %d", s, g.dirtLevel(0))
		}
		if n := len(g.spots(0)); n != oldSpots(s) {
			t.Fatalf("dirt %d: %d spots, trunk had %d", s, n, oldSpots(s))
		}
		if g.closed(0) != (s >= dirtMax) || g.rotting(0) != (m <= rotMood) {
			t.Fatalf("dirt %d: closed %v rotting %v", s, g.closed(0), g.rotting(0))
		}
		if g.halveDirt(0); g.dirtLevel(0) != s/2 {
			t.Fatalf("dirt %d: a Priest leaves %d, trunk left %d", s, g.dirtLevel(0), s/2)
		}
		// the harvest: one clean in ghost banks half the spiritual dirt it held
		g.mood[0], g.ghost, g.wallet = m, true, [2]Num{}
		g.dirt = nil
		for _, sp := range g.spots(0) {
			g.dirt[0][slices.Index(g.dirt[0], sp)].spirit = true
		}
		if s > 0 {
			g.clean(0, g.dirt[0][0].cell)
		}
		if g.wallet[Ecto] != Num(s)*harvestRate || g.roomMood(0) != moodClean || len(g.spots(0)) != 0 {
			t.Fatalf("dirt %d: cleaned for %v Ecto, Mood %d; want %v and %d", s, g.wallet[Ecto], g.roomMood(0), Num(s)*harvestRate, moodClean)
		}
		g.ghost = false
	}
	if g.addMood(0, -500); g.roomMood(0) != moodMin {
		t.Fatalf("addMood past the floor: %d", g.roomMood(0))
	}
	if g.addMood(0, 500); g.roomMood(0) != moodMax {
		t.Fatalf("addMood past the ceiling: %d", g.roomMood(0))
	}
}

// lonePartyAt puts a one-Guest Party of the realm into Room room, walking the Corridor from `from` to `to`.
func lonePartyAt(g *Game, realm Realm, room int, from, to image.Point) *entities.Guest {
	guest := entities.NewGuest(from, guestCast[0])
	guest.RoomIndex = room
	guest.SetPath([]image.Point{to})
	g.parties = append(g.parties, &Party{Guests: []*entities.Guest{&guest}, state: staying,
		Booking: Booking{Room: room, Realm: realm, Nights: 1, Checkout: 1 << 30}})
	return &guest
}

// T251: a Party lives in one realm. A human and a ghost Party booked into one Room each occupy only their own
// layer, and walking head-on in the Corridor they pass through each other, where two humans would sidestep.
func TestRealms(t *testing.T) {
	g, _, human := pairIn(t, 2, 1, 0)
	ghost := &Party{Guests: human.Guests[:1], state: staying, Booking: Booking{Room: 0, Realm: Ghost, Nights: 1, Checkout: 1 << 30}}
	if human.Realm != Human {
		t.Fatalf("a Party is %v by default, want Human", human.Realm)
	}
	g.parties = append(g.parties, ghost)
	if g.vacant(0, Human) || g.vacant(0, Ghost) || !human.Occupies(0, Human) || human.Occupies(0, Ghost) || ghost.Occupies(0, Human) || !ghost.Occupies(0, Ghost) {
		t.Fatal("each Party must occupy only its own layer of the Room")
	}
	if g.parties = g.parties[1:]; !g.vacant(0, Human) || g.vacant(0, Ghost) {
		t.Fatal("with only the ghost booked, the human layer must be vacant and the ghost layer not")
	}
	g.parties = nil
	if !g.vacant(0, Human) || !g.vacant(0, Ghost) {
		t.Fatal("an empty Room must be vacant in both layers")
	}
	if g.realm() != Human {
		t.Fatal("the Concierge is Human until he shifts")
	}
	if g.ghost = true; g.realm() != Ghost {
		t.Fatal("the Concierge is in Ghost while g.ghost")
	}

	r := g.hotel.Rooms[0]
	y := r.Door.Y + 3
	cell := func(dx int) image.Point {
		return image.Pt((r.Door.X+dx)*world.TileSize+world.TileSize/2, y*world.TileSize+world.TileSize/2)
	}
	if g.hotel.Map.Solid(r.Door.X-2, y) || g.hotel.Map.Solid(r.Door.X+6, y) {
		t.Fatal("the Corridor test row isn't floor")
	}
	pass := func(ra, rb Realm) (detour float64, done bool) {
		g.parties, g.ghost = nil, false
		g.player.X, g.player.Y = float64(g.hotel.ConciergeStart.X), float64(g.hotel.ConciergeStart.Y)
		a, b := lonePartyAt(g, ra, 0, cell(-2), cell(6)), lonePartyAt(g, rb, 0, cell(6), cell(-2))
		y0 := a.Y
		for range 10 * tps {
			g.walk(a, arriveSpeed)
			g.walk(b, arriveSpeed)
			g.separate()
			detour = max(detour, math.Abs(a.Y-y0), math.Abs(b.Y-y0))
			if len(a.Path) == 0 && len(b.Path) == 0 {
				return detour, true
			}
		}
		return detour, false
	}
	if d, done := pass(Human, Human); d == 0 && done {
		t.Fatal("two humans met head-on without a sidestep: the test row doesn't test the bump")
	}
	if d, done := pass(Human, Ghost); d != 0 || !done {
		t.Fatalf("a human and a ghost met head-on: detour %.1f px, done %v; want to pass straight through", d, done)
	}
}

// T252: a Guest flees at its own Fear limit.
func TestFleeAt(t *testing.T) {
	g, _, p := pairIn(t, 2, 1, 0)
	if p.Guests[0].FleeAt != fearLimit {
		t.Fatalf("FleeAt defaults to %d, want fearLimit %d", p.Guests[0].FleeAt, fearLimit)
	}
	p.Guests[0].FleeAt = 50
	p.Guests[0].Fear = 49
	if g.scare(); p.scares != 0 {
		t.Fatal("a Party stepped out under its Guest's FleeAt")
	}
	p.Guests[0].Fear = 50
	if g.scare(); p.scares != 1 || p.state != cooling {
		t.Fatalf("at Fear 50 with FleeAt 50: scares %d, state %d; want 1 and cooling", p.scares, p.state)
	}
	if p.Guests[0].Phantasm != 0 {
		t.Fatal("Phantasm starts at 0")
	}
	if p.Guests[0].Haunt != entities.DefaultHaunt {
		t.Fatal("Haunt starts at its default")
	}
}

// T253: g.actors() walks every actor on the map once: Party members, couriers and Priests; the Concierge and
// the Shadows are handled apart.
func TestActors(t *testing.T) {
	g, _, p := pairIn(t, 2, 1, 0)
	g.placeShadow(1)
	c := &courier{Guest: entities.NewGuest(g.hotel.Spawn, courierName)}
	g.couriers = append(g.couriers, c)
	g.sendPriest(1, false)
	g.parties = append(g.parties, &Party{Guests: []*entities.Guest{{Character: "Amelia"}}, state: out, Booking: Booking{Room: 1}}) // off the map
	got := g.actors()
	want := []*entities.Guest{p.Guests[0], p.Guests[1], &c.Guest, &g.priests[0].Guest}
	if len(got) != len(want) {
		t.Fatalf("%d actors, want %d", len(got), len(want))
	}
	for _, w := range want {
		if n := slices.Index(got, w); n < 0 || slices.Index(got[n+1:], w) >= 0 {
			t.Fatalf("actor %q counted %d times, want once", w.Character, slices.Index(got, w))
		}
	}
}
