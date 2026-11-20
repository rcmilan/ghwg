package game

import (
	"reflect"
	"testing"

	"ghosthotel/entities"
)

// who: the identities of n ghost and n human guests a Game rolls, in order.
func who(t *testing.T, seed uint64, n int) (ids []entities.Identity) {
	g, _ := headless(t, 2, 1, seed)
	g.grid0 = [2]int{1, 1} // as if the hotel had grown: quirks roll only then (T376)
	for range n {
		h := entities.NewGuest(g.hotel.Spawn, "Adam")
		g.rollHuman(&h)
		gh := g.newGhost()
		ids = append(ids, h.Who, gh.Who)
	}
	return ids
}

func TestIdentityDeterministicPerSeed(t *testing.T) {
	a, b, c := who(t, 5, 40), who(t, 5, 40), who(t, 6, 40)
	if !reflect.DeepEqual(a, b) {
		t.Error("the same seed must roll the same guests")
	}
	if reflect.DeepEqual(a, c) {
		t.Error("another seed rolled the same guests")
	}
}

func TestIdentityShape(t *testing.T) {
	quirks, vegs, humans := 0, 0, 0
	for i, id := range who(t, 3, 600) {
		if id.Name == "" || id.Card == "" || id.Key != "" {
			t.Fatalf("guest %d: bad identity %+v", i, id)
		}
		if i%2 == 0 { // human
			humans++
			if id.Age < humanAgeMin || id.Age > humanAgeMax || id.Job == "" || id.Died != 0 || id.Quirk > NeatFreak {
				t.Fatalf("human %d: %+v", i, id)
			}
			vegs += b2i(id.Vegetarian)
		} else if id.Died < ghostDiedMin || id.Died > ghostDiedMax || id.Cause == "" || id.Age != 0 || id.Job != "" || id.Vegetarian || (id.Quirk != NoQuirk && id.Quirk < ShowOff) {
			t.Fatalf("ghost %d: %+v", i, id)
		}
		quirks += b2i(id.Quirk != NoQuirk)
	}
	if quirks < 300 || quirks > 500 { // about 1 in 3 of 1200
		t.Errorf("%d quirks in 1200 guests, want about 400", quirks)
	}
	if vegs < 90 || vegs > 150 { // about 1 in 5 of 600
		t.Errorf("%d vegetarians in 600 humans, want about 120", vegs)
	}
}

// Rolling identities never draws from rng, chat or words: the simulation is the one it was.
func TestIdentityLeavesOtherRngs(t *testing.T) {
	a, _ := headless(t, 2, 1, 9)
	b, _ := headless(t, 2, 1, 9)
	h := entities.NewGuest(a.hotel.Spawn, "Adam")
	for range 20 {
		a.rollHuman(&h)
		a.rollGhost(&h)
	}
	if a.rng.Uint64() != b.rng.Uint64() || a.chat.Uint64() != b.chat.Uint64() || a.words.Uint64() != b.words.Uint64() {
		t.Error("an identity roll moved rng, chat or words")
	}
}

// A booking rolls every guest of the Party, in both realms, and the Party starts unrevealed.
func TestBookingRollsIdentity(t *testing.T) {
	g, _ := headless(t, 2, 1, 4)
	g.unlock(Mirror)
	p := g.arrive(0)
	q := g.arriveGhost(0)
	for _, party := range []*Party{p, q} {
		if party.revealed {
			t.Error("a new Party is not revealed")
		}
		for _, guest := range party.Guests {
			if guest.Who.Name == "" || guest.Who.Card == "" {
				t.Errorf("realm %v: a guest has no identity: %+v", party.Realm, guest.Who)
			}
		}
	}
}

func TestCastData(t *testing.T) {
	if len(quirkNames) != int(Homebody)+1 {
		t.Fatal("a quirk has no name")
	}
	if len(namedHumans) != 7 || len(namedGhosts) != 3 {
		t.Errorf("%d named human entries and %d ghosts, want 7 and 3 (cast.md says 6 entries, but lists 7)", len(namedHumans), len(namedGhosts))
	}
	kinds := map[string]bool{}
	for _, k := range ghostKinds {
		kinds[k.Name] = true
	}
	for _, h := range namedHumans {
		skeptic := h.Quirk == Skeptic
		if h.Key == "" || len(h.Members) == 0 || h.Dread == "" || (h.Fled == "") != skeptic || h.Quirk > NeatFreak {
			t.Errorf("named human %s is incomplete: %+v", h.Key, h)
		}
		for _, v := range h.Visit {
			if v == "" {
				t.Errorf("%s misses a visit line", h.Key)
			}
		}
	}
	if len(namedHumans[4].Members) != 2 {
		t.Error("Bea and Lou are one Party of two")
	}
	for _, gh := range namedGhosts {
		if gh.Key == "" || gh.Died == 0 || !kinds[gh.Kind] || gh.Fact == "" || gh.Card == "" || gh.Quirk < ShowOff {
			t.Errorf("named ghost %s is incomplete: %+v", gh.Key, gh)
		}
	}
	if len(humanFirst) != 20 || len(humanLast) != 20 || len(humanJobs) != 20 || len(ghostFirst) != 20 || len(ghostLast) != 20 || len(ghostCause) != 10 || len(humanCards) != 6 || len(ghostCards) != 6 {
		t.Error("a generated table has the wrong size")
	}
}
