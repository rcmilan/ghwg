package game

import (
	"image"
	"math/rand/v2"
	"slices"
	"testing"

	"ghosthotel/entities"
)

// 08 W5 R2: each quest character has sheets of its own in assets (tools/questlooks): none is its base's nor
// another's, the Game loads them as drawn, and a guest named with that Key wears them, no random look on top.
func TestQuestLooksLoadTheirOwnSheets(t *testing.T) {
	keys := []string{"magenta", "celestine", "magda", "tim", "teo", "osvaldo"}
	if len(questLooks) != len(keys) || len(questBases) != len(keys) {
		t.Fatalf("%d looks, %d bases, want %d of each", len(questLooks), len(questBases), len(keys))
	}
	g, _ := headless(t, 2, 1, 1)
	g.startRun()
	bases, err := loadCharacters(guestCast[:]...)
	if err != nil {
		t.Fatal(err)
	}
	worn := questSheets(bases)
	for name, base := range questBases { // with its files missing, a look is its base's sheets
		if gone := questSheets(map[string][2]*image.RGBA{base: bases[base]}); questBases[name] != base || gone[name] == ([2]*image.RGBA{}) {
			t.Fatalf("%s has no sheets to fall back on", name)
		}
	}
	seen := map[string]string{}
	for _, key := range keys {
		name := questLooks[key]
		base := questBases[name]
		sheets, err := loadCharacters(name, base)
		if err != nil {
			t.Fatalf("%s: %v", key, err)
		}
		for i, s := range sheets[name] {
			if s.Bounds() != sheets[base][i].Bounds() {
				t.Errorf("%s sheet %d is %v, its base's %v", name, i, s.Bounds(), sheets[base][i].Bounds())
			}
			if string(s.Pix) == string(sheets[base][i].Pix) {
				t.Errorf("%s sheet %d is %s's", name, i, base)
			}
			if other, dup := seen[string(s.Pix)]; dup {
				t.Errorf("%s sheet %d is %s's", name, i, other)
			}
			seen[string(s.Pix)] = name
			if string(worn[name][i].Pix) != string(s.Pix) {
				t.Errorf("the Game's %s sheet %d is not the asset", name, i)
			}
		}
		if slices.Contains(guestCast[:], name) || slices.Contains(shadowCast, name) {
			t.Errorf("%s: in the walk-ins' cast %v, in the ghosts' %v", name, slices.Contains(guestCast[:], name), slices.Contains(shadowCast, name))
		}
	}
	// named, each wears its look: the humans through the book's roll and the quest's check-in, the ghosts through nameGhost
	p := g.walkIn()
	for _, h := range namedHumans {
		want, quest := questLooks[h.Key]
		p.Guests = nil
		for range h.Members { // fresh guests, as a Party that just booked
			guest := entities.NewGuest(g.hotel.Spawn, g.dressed("Adam"))
			p.Guests = append(p.Guests, &guest)
		}
		g.nameHuman(p, h)
		if got := p.Guests[0].Character; quest && got != want {
			t.Errorf("%s wears %q, want %q", h.Key, got, want)
		} else if base, _, cut := cutLook(got); !quest && (!cut || !slices.Contains(guestCast[:], base)) {
			t.Errorf("%s, no quest character, wears %q, want an ordinary sheet under a random look", h.Key, got)
		}
	}
	for _, n := range namedGhosts {
		var ghost entities.Guest
		ghost.Character = "Adam"
		if g.nameGhost(&ghost, n); ghost.Character != questLooks[n.Key] {
			t.Errorf("%s wears %q, want %q", n.Key, ghost.Character, questLooks[n.Key])
		}
		if _, ok := questHues[n.Key]; !ok {
			t.Errorf("%s has no hue of its own in ghost", n.Key)
		}
	}
	if q := g.placeNamed(1, "magda"); q == nil || q.Guests[0].Character != nameMagda {
		t.Errorf("the quest's check-in: Magda wears %v", q)
	}
}

func cutLook(character string) (base, seed string, ok bool) {
	for i := range character {
		if character[i] == '#' {
			return character[:i], character[i+1:], true
		}
	}
	return character, "", false
}

// A quest ghost's hue in ghost is its own: no kind's, no other quest ghost's, not a human's white.
func TestQuestHuesAreTheirOwn(t *testing.T) {
	g, _ := headless(t, 2, 1, 1)
	g.startRun()
	taken := [][3]float32{{1, 1, 1}}
	for _, k := range ghostKinds {
		taken = append(taken, k.Hue)
	}
	for _, n := range namedGhosts {
		p := ghostIn(t, g, 0, 5)
		g.nameGhost(p.Guests[0], n)
		tint, _, _ := g.ghostLook(p.Guests[0])
		hue := [3]float32{tint[0], tint[1], tint[2]}
		if hue != questHues[n.Key] || slices.Contains(taken, hue) {
			t.Errorf("%s is drawn in %v, want its own %v, unlike %v", n.Key, hue, questHues[n.Key], taken)
		}
		taken = append(taken, hue)
		g.parties = nil
	}
}

// Ordinary guests never get a quest palette: no random look pairs a quest character's hair and top, and the
// quest sheets are never dressed in one (TestQuestLooksLoadTheirOwnSheets: they are nobody's base).
func TestOrdinaryGuestsNeverWearAQuestPalette(t *testing.T) {
	rolled := map[[2]uint32]bool{}
	for seed := range uint64(20000) {
		l := randomLook(rand.New(rand.NewPCG(seed, 0)))
		if questPair(l) {
			t.Fatalf("seed %d: hair #%06x over top #%06x is a quest character's", seed, l[hair], l[top])
		}
		rolled[[2]uint32{l[hair], l[top]}] = true
	}
	if want := len(hairs)*len(tops) - len(questPairs); len(rolled) != want {
		t.Errorf("%d hair and top pairs rolled, want every other one: %d", len(rolled), want)
	}
	for _, pair := range questPairs {
		if !slices.Contains(hairs[:], pair[0]) || !slices.Contains(tops[:], pair[1]) {
			t.Errorf("the pair #%06x, #%06x is not of the ordinary ramps", pair[0], pair[1])
		}
	}
}

// Magenta is never scared: a Shadow she sees gives her no Fear, no line and no hop, and pays nothing; a ghost
// beyond her Nerve does not terrify her; at any Fear she neither steps out nor flees. The guest beside her is.
func TestMagentaIsNeverScared(t *testing.T) {
	g, _, h := settled(t)
	g.locked = nil
	magenta := h.Guests[0]
	g.nameHuman(h, namedHumans[slices.IndexFunc(namedHumans, func(n namedHuman) bool { return n.Key == "magenta" })])
	if magenta.Who.Key != "magenta" || magenta.Character != nameMagenta {
		t.Fatalf("named %q, wearing %q", magenta.Who.Key, magenta.Character)
	}
	ghost := ghostIn(t, g, h.Room, phantasmStrong)
	setNerve(h, phantasmMax)
	run, xa, _ := g.backWall(h.Room)
	ecto := g.wallet[Ecto]
	for range 20 {
		magenta.Path, magenta.Anim.Facing, h.talk = nil, entities.FaceUp, nil
		sh := &shade{run: run, x: xa, seen: map[*entities.Guest]int{}}
		g.sighting(h, h.Guests, sh)
		if _, hopped := sh.seen[magenta]; hopped || len(h.talk) != 0 {
			t.Fatalf("a sighting: hopped %v, said %v", hopped, h.talk)
		}
	}
	g.frighten(h, magenta, 1000, 1<<20)
	setNerve(h, 1) // the ghost is far beyond it
	for range 5 {
		g.terrify(h.Room, ghost.Guests[0])
	}
	if magenta.Fear != 0 || g.wallet[Ecto] != ecto {
		t.Fatalf("Fear %d, Ecto %v → %v; want none of either", magenta.Fear, ecto, g.wallet[Ecto])
	}
	magenta.Fear = magenta.FleeAt // however it got there
	if g.mayFlee(h, magenta) {
		t.Fatal("at her limit she may flee")
	}
	for range 3 * minuteFrames {
		g.step(0, 0)
	}
	if h.Fled || h.state != staying || h.scares != 0 {
		t.Fatalf("at her limit: fled %v, state %d, scares %d", h.Fled, h.state, h.scares)
	}
	// anyone else in her place is frightened by the same
	g2, _, o := settled(t)
	g2.locked = nil
	other := o.Guests[0]
	ghostIn(t, g2, o.Room, phantasmStrong)
	setNerve(o, phantasmMax)
	run, xa, _ = g2.backWall(o.Room)
	other.Path, other.Anim.Facing = nil, entities.FaceUp
	g2.sighting(o, o.Guests, &shade{run: run, x: xa, seen: map[*entities.Guest]int{}})
	if other.Fear == 0 {
		t.Fatal("an ordinary guest seeing the same Shadow took no Fear: the test frightens no one")
	}
}
