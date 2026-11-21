package game

import (
	"image"
	"slices"
	"strings"
	"testing"

	"ghosthotel/entities"
	"ghosthotel/world"

	"github.com/hajimehoshi/ebiten/v2"
)

// 08 W5-VERBS. QUEST is not part of this slice: the tests set g.quest by hand and read what the verbs reported
// (g.verbs.done, the objectives told to questEvent through verbDone).

// verbRun: a Run of cols Rooms with the questline at beat (its objectives all open).
func verbRun(t *testing.T, cols, beat int) (*Game, *script) {
	t.Helper()
	g, s := matchRun(t, cols)
	setQuest(g, beat)
	return g, s
}

func setQuest(g *Game, beat int) {
	g.quest = questRun{Beat: beat, Done: make([]bool, len(beats[beat-1].Objs)), Rolls: map[string]int{}}
	g.verbs = verbState{beat: beat}
}

// flat: the Card's text on one line, wraps undone.
func flat(g *Game) string { return strings.Join(strings.Fields(cardText(g)), " ") }

func reported(g *Game, id string) bool { return slices.Contains(g.verbs.done, id) }

func optsOf(g *Game, p *Party) (os []cardOpt) { return g.cardOpts(p, p.Guests[0]) }

func hasOpt(os []cardOpt, o cardOpt) bool { return slices.Contains(os, o) }

// named places the named guest in the Room, failing the test if it can't.
func named(t *testing.T, g *Game, room int, key string) *Party {
	t.Helper()
	p := g.placeNamed(room, key)
	if p == nil {
		t.Fatalf("no %s", key)
	}
	return p
}

// minuteAt runs the game minute at clock c.
func minuteAt(g *Game, c int) {
	g.clock, g.ticked = c, true
	g.verbMinute()
}

// O1: Morwenna's Card in real gives the word; it crosses the SHIFT, and TELL shows only on the ghost roommate's Card.
func TestO1TellAcrossShift(t *testing.T) {
	g, s := verbRun(t, 2, 1)
	m := named(t, g, 0, "morwenna")
	if g.verbWait("O1") != true {
		t.Error("no ghost in her Room, but the row does not wait")
	}
	ghostOf(g, 0, 3)
	if g.verbWait("O1") {
		t.Error("a ghost is in her Room, and the row still waits")
	}
	if hasOpt(optsOf(g, m), optTell) {
		t.Error("TELL shows with no word held")
	}
	press(g, s, center(g.hotel.Rooms[0].Plate()))
	if g.state != StateTalk || g.verbs.held != "O1" {
		t.Fatalf("her Card: state %d, held %q; want the Card open and the word held", g.state, g.verbs.held)
	}
	if txt := flat(g); !strings.Contains(txt, "witch has arrived") {
		t.Errorf("her Card does not say the Ask:\n%s", txt)
	}
	if hasOpt(optsOf(g, m), optTell) {
		t.Error("TELL shows on Morwenna's own Card")
	}
	keys(g, s, ebiten.KeyEscape)
	g.ghost = true // SHIFT: the word stays
	if g.verbs.held != "O1" {
		t.Fatal("the word did not cross the SHIFT")
	}
	press(g, s, center(g.hotel.Rooms[0].Plate()))
	if g.parley == nil || g.parley.card == nil || !hasOpt(g.cardOpts(g.parley.p, g.parley.p.Guests[0]), optTell) {
		t.Fatal("no TELL on the ghost roommate's Card")
	}
	tap(g, s, ebiten.KeyS)
	if !strings.Contains(flat(g), "TELL THE WORD") {
		t.Errorf("the TELL row:\n%s", flat(g))
	}
	tap(g, s, ebiten.KeySpace)
	if !reported(g, "O1") || g.verbs.held != "" {
		t.Errorf("TELL: reported %v, held %q", g.verbs.done, g.verbs.held)
	}
	if txt := flat(g); !strings.Contains(txt, "A witch! Finally") {
		t.Errorf("the ghost does not answer:\n%s", txt)
	}
}

// Menus show only what applies: no TELL, TAKE or GIVE without a word or thing for that guest, none for a closed objective.
func TestVerbMenusOnlyWhatApplies(t *testing.T) {
	g, _ := verbRun(t, 3, 1)
	m, ghost := named(t, g, 0, "morwenna"), ghostOf(g, 0, 3)
	for _, held := range []string{"", "O2", "O9", "O11"} {
		g.verbs.held = held
		for _, p := range []*Party{m, ghost} {
			for _, o := range optsOf(g, p) {
				if o >= optTell {
					t.Errorf("held %q: a quest option %d on %v's Card", held, o, p.Guests[0].Who.Name)
				}
			}
		}
	}
	g.verbs.held = "O1"
	if !hasOpt(optsOf(g, ghost), optTell) || hasOpt(optsOf(g, m), optTell) {
		t.Error("held O1: TELL should show on the ghost only")
	}
	g.quest.Done[0] = true
	if hasOpt(optsOf(g, ghost), optTell) {
		t.Error("TELL shows with O1 done")
	}
}

// The MacGuffin is not REVEALable: its Card has the "IT REVEALS YOU" row and no REVEAL.
func TestMacGuffinHasNoReveal(t *testing.T) {
	g, _ := verbRun(t, 2, 1)
	p := settledHuman(t, g, 0, 5)
	p.Guests[0].Who.Key = "macguffin"
	os := optsOf(g, p)
	if hasOpt(os, optReveal) || !hasOpt(os, optNoReveal) {
		t.Fatalf("the MacGuffin's options: %v", os)
	}
	g.startCard(0)
	if txt := flat(g); !strings.Contains(txt, macguffinCard.Reveal) || strings.Contains(txt, "REVEAL 10") {
		t.Errorf("the MacGuffin's Card:\n%s", txt)
	}
}

// O2: Celestine's entry is in the book only in ghost; reading it hands the word; TELL Morwenna in real. Lv2+ needs the
// second part too.
func TestO2BookEntryGhostOnly(t *testing.T) {
	for _, level := range []int{0, 2} {
		g, s := verbRun(t, 2, 2)
		g.level = level
		setQuest(g, 2)
		named(t, g, 1, "celestine")
		m := named(t, g, 0, "morwenna")
		want := g.fill(celestineEntry, 1)
		if strings.Contains(entryLines(g), want) {
			t.Fatal("real: the book shows her entry")
		}
		g.openBook()
		if g.verbs.held != "" {
			t.Error("real: reading the book gave a word")
		}
		g.state, g.ghost = StatePlay, true
		g.openBook()
		if !strings.Contains(entryLines(g), want) {
			t.Fatalf("ghost: no entry %q in %q", want, entryLines(g))
		}
		if level == 2 && !strings.Contains(entryLines(g), celestineEntryHard) {
			t.Error("Lv2: no continuation")
		}
		if g.verbs.held != "O2" {
			t.Fatalf("level %d: the word is %q after reading", level, g.verbs.held)
		}
		g.state, g.ghost = StatePlay, false
		if !hasOpt(optsOf(g, m), optTell) {
			t.Fatal("no TELL on Morwenna's Card")
		}
		g.startCard(0)
		g.cardVerb(optTell, m, m.Guests[0], g.parley.card)
		if !reported(g, "O2") || !strings.Contains(flat(g), "Ninety-seven years") {
			t.Errorf("level %d: reported %v:\n%s", level, g.verbs.done, flat(g))
		}
		_ = s
	}
}

func entryLines(g *Game) string {
	var ls []string
	for _, e := range g.shown() {
		ls = append(ls, e.Line)
	}
	return strings.Join(ls, "|")
}

// The talk's target override, shared by O3, O7 and O10: the target is first, the only offer is the discount, and sure.
func TestTargetOverride(t *testing.T) {
	g, _ := verbRun(t, 3, 3)
	m := named(t, g, 0, "magda")
	setNerve(m, 2)
	cel := named(t, g, 1, "celestine")
	cel.Guests[0].Phantasm = 10
	g.wallet[Money] = 1000
	if slices.Contains(g.targetsPlain(m), 1) {
		t.Fatal("the bad pair is a plain target: the override proves nothing")
	}
	ts := g.targets(m)
	if len(ts) == 0 || ts[0] != 1 {
		t.Fatalf("targets %v: Madame's Room first wanted", ts)
	}
	if os := g.offers(m, 1); len(os) != 1 || os[0] != discount {
		t.Errorf("offers %v: want the discount only", os)
	}
	if g.offerOdds(m, 1, discount) != 1 {
		t.Error("the target's offer is not sure")
	}
	if to, _, _, ok := g.sureOffer(m); !ok || to != 1 {
		t.Errorf("sureOffer %d %v", to, ok)
	}
	g.startTalk(m)
	g.parley.pick = discount
	g.makeOffer()
	if m.Room != 1 {
		t.Fatalf("Magda is in Room %d after the sure offer", m.Room)
	}
	if g.wallet[Money] != 1000-g.offerCost(m, 0, discount) && g.wallet[Money] >= 1000 {
		t.Errorf("the offer cost nothing: %v", g.wallet[Money])
	}
	g.quest.Done[0] = true // O3 done: no override
	if q := g.questTarget(m); q >= 0 {
		t.Errorf("target %d with O3 done", q)
	}
}

// targetsPlain: targets without the override, for the test that it matters.
func (g *Game) targetsPlain(p *Party) []int {
	q := g.quest
	g.quest = questRun{}
	defer func() { g.quest = q }()
	return g.targets(p)
}

// O3: she stays the night: the dawn finishes it, a day she was moved in late does not.
func TestO3DawnNeedsTheNight(t *testing.T) {
	g, _ := verbRun(t, 3, 3)
	m := named(t, g, 0, "magda")
	named(t, g, 1, "celestine")
	g.relocate(m, 1)
	g.settleIn(m)
	minuteAt(g, dayFrames+runStart)
	if reported(g, "O3") {
		t.Fatal("done at the first dawn, with no night in her")
	}
	minuteAt(g, 22*hourFrames)
	minuteAt(g, dayFrames+runStart)
	if !reported(g, "O3") {
		t.Fatal("not done after the night")
	}
}

// O3 Lv2+: a stranger holds Madame's Room and is pinned until Magda is in.
func TestO3StrangerLv2(t *testing.T) {
	g, _ := verbRun(t, 3, 3)
	g.level = 2
	m := named(t, g, 0, "magda")
	cel := named(t, g, 1, "celestine")
	g.verbMinute()
	st := g.verbs.stranger
	if st == nil || st.Room != 1 || !g.verbPinned(st) {
		t.Fatalf("stranger %v: wanted a pinned human in Madame's Room", st)
	}
	if ts := g.targets(m); len(ts) == 0 || ts[0] != 1 {
		t.Errorf("the override with the stranger there: %v", ts)
	}
	g.relocate(m, 1) // a swap: the stranger goes to Magda's
	if g.verbPinned(st) {
		t.Error("the stranger is pinned after it was moved out")
	}
	_ = cel
}

// O4: the spirit dashes at night; the beam lights it, and a Concierge facing away lights nothing.
func spiritRun2(t *testing.T, level int) *Game {
	g, _ := verbRun(t, 3, 3)
	g.level = level
	g.rollSet("O4.room", 1)
	g.rollSet("O4.door", 2)
	g.clock = 23 * hourFrames
	return g
}

// lane: the spirit's Corridor.
func lane(g *Game) (x0, x1, y float64) { return g.spiritLane(g.spiritRow()) }

func TestO4SpiritLitByTheBeam(t *testing.T) {
	g := spiritRun2(t, 0)
	x0, x1, y := lane(g)
	g.player.X, g.player.Y = (x0+x1)/2, y
	g.player.Anim.Facing = entities.FaceLeft // the spirit comes from the west
	for i := 0; i < 600 && g.rollGet("O4.lit") == 0; i++ {
		g.step(0, 0)
	}
	if g.rollGet("O4.lit") != 1 {
		t.Fatal("the spirit was never lit")
	}
	if g.handprintRoom() != 1 {
		t.Fatalf("handprint on Room %d, wanted the rolled door 1", g.handprintRoom())
	}
	if reported(g, "O4") {
		t.Fatal("O4 reported before the door")
	}
	if g.scene.on() { // the O4 cutscene stops the world: it plays out first
		g.endScene()
	}
	g.ghost = true // SHIFT: SPACE on the door
	s := &script{}
	g.in = s
	press(g, s, center(g.hotel.Rooms[1].Mat()))
	if !reported(g, "O4") {
		t.Error("SPACE on the handprint door did not finish O4")
	}
}

func TestO4SpiritMissedWhenFacingAway(t *testing.T) {
	g := spiritRun2(t, 0)
	x0, x1, y := lane(g)
	g.player.X, g.player.Y = (x0+x1)/2, y
	g.player.Anim.Facing = entities.FaceDown // the beam points at the wall
	for i := 0; i < 400; i++ {
		g.step(0, 0)
	}
	if g.rollGet("O4.lit") != 0 {
		t.Error("lit with the beam pointing away")
	}
	if g.verbs.spirit.next == 0 {
		t.Error("the spirit did not finish its dash and wait for the next")
	}
}

func TestO4SpiritByDay(t *testing.T) {
	g := spiritRun2(t, 0)
	g.clock = 12 * hourFrames
	for range 100 {
		g.step(0, 0)
	}
	if g.verbs.spirit.on {
		t.Error("the spirit dashes in daylight")
	}
}

func TestO4TwoDashesBothLitAtLv2(t *testing.T) {
	g := spiritRun2(t, 2)
	x0, x1, y := lane(g)
	g.player.X, g.player.Y = (x0+x1)/2, y
	for i := 0; i < 900 && g.rollGet("O4.lit") == 0; i++ {
		g.player.Anim.Facing = entities.FaceLeft // dash 0 comes from the west, dash 1 from the east
		if g.verbs.spirit.dash == 1 {
			g.player.Anim.Facing = entities.FaceRight
		}
		g.step(0, 0)
	}
	if g.rollGet("O4.lit") != 1 {
		t.Fatalf("both dashes lit, still not caught: %+v", g.verbs.spirit)
	}
	// and only one lit: it tries again later
	g2 := spiritRun2(t, 2)
	g2.player.X, g2.player.Y = (x0+x1)/2, y
	for i := 0; i < 900 && g2.verbs.spirit.next == 0; i++ {
		g2.player.Anim.Facing = entities.FaceLeft // dash 0 lit, dash 1 missed
		if g2.verbs.spirit.dash == 1 {
			g2.player.Anim.Facing = entities.FaceDown
		}
		g2.step(0, 0)
	}
	if g2.rollGet("O4.lit") != 0 || g2.verbs.spirit.next == 0 {
		t.Errorf("one lit dash of two counted: lit %d next %d", g2.rollGet("O4.lit"), g2.verbs.spirit.next)
	}
}

// O5: only a quest ghost asks; Ned asks for a Supply, fetched in real and given at his door in ghost.
func TestO5GhostRequest(t *testing.T) {
	g, s := verbRun(t, 2, 4)
	ned := named(t, g, 0, "ned")
	other := ghostOf(g, 1, 3)
	for range 3 * dayFrames / minuteFrames {
		g.clock += minuteFrames
		g.updateAsks()
	}
	if other.asking() || ned.asking() {
		t.Fatal("a ghost asked through the ordinary asks")
	}
	g.clock = runStart
	minuteAt(g, runStart)
	if !ned.asking() || other.asking() {
		t.Fatalf("Ned asks %v, another ghost %v", ned.asking(), other.asking())
	}
	want := ned.want
	g.ghost, g.hands = true, SupplyFood
	if want == SupplyFood {
		g.hands = SupplyWater
	}
	press(g, s, center(g.hotel.Rooms[0].Mat()))
	if reported(g, "O5") || !ned.asking() {
		t.Fatal("the wrong Supply was taken")
	}
	g.state = StatePlay // SPACE with nothing to do opened the hotel view
	g.hands = want
	press(g, s, center(g.hotel.Rooms[0].Mat()))
	if !reported(g, "O5") || ned.asking() || g.hands != "" {
		t.Errorf("delivered: reported %v asking %v hands %q", g.verbs.done, ned.asking(), g.hands)
	}
}

func TestO5TwoAsksAtLv2(t *testing.T) {
	g, s := verbRun(t, 2, 4)
	g.level = 2
	ned := named(t, g, 0, "ned")
	g.ghost = true
	minuteAt(g, runStart)
	first := ned.want
	g.hands = first
	press(g, s, center(g.hotel.Rooms[0].Mat()))
	if reported(g, "O5") {
		t.Fatal("done after one of two")
	}
	g.state = StatePlay
	minuteAt(g, runStart+minuteFrames)
	if !ned.asking() || ned.want == first {
		t.Fatalf("no second, different ask: asking %v %q after %q", ned.asking(), ned.want, first)
	}
	g.hands = ned.want
	press(g, s, center(g.hotel.Rooms[0].Mat()))
	if !reported(g, "O5") {
		t.Error("not done after two")
	}
}

// O6: Teo names a Room; its ghost reads NIGHT PORTER; Father K's Ritual there is the party that finishes it.
func TestO6RetirementParty(t *testing.T) {
	g, _ := verbRun(t, 3, 4)
	teo := named(t, g, 0, "teo")
	porter := ghostOf(g, 1, 3)
	g.verbMinute()
	if g.porterRoom() != 1 || g.porterParty() != porter {
		t.Fatalf("porter room %d", g.porterRoom())
	}
	if !g.verbPinned(porter) {
		t.Error("the porter is not held")
	}
	g.startCard(0)
	if txt := flat(g); !strings.Contains(txt, "Room 102") {
		t.Errorf("Teo's Card does not name the Room:\n%s", txt)
	}
	g.endTalk()
	_ = teo
	g.ghost = true
	g.wallet[Ecto] = 50
	g.reveal(porter)
	g.startCard(1)
	if txt := flat(g); !strings.Contains(txt, porterQuirk) || !strings.Contains(txt, "Clocked in 1923") {
		t.Errorf("the porter's read:\n%s", txt)
	}
	g.endTalk()
	pr := &priest{}
	pr.RoomIndex = 1
	if l := g.verbOpener(pr); len(l) != 1 || l[0] != fatherKParty[0] {
		t.Errorf("K's opener: %v", l)
	}
	g.questRec.Times[3] = 1
	if l := g.verbOpener(pr); l[0] != fatherKParty[1] {
		t.Errorf("K's opener the 2nd time: %v", l)
	}
	g.ritualEffect(&priest{strong: true, Guest: entities.Guest{RoomIndex: 1}}) // M's Ritual: no party
	if reported(g, "O6") {
		t.Fatal("Father M finished O6")
	}
	g.ritualEffect(pr)
	if !reported(g, "O6") || porter.Guests[0].PassAt == 0 {
		t.Errorf("K's Ritual: reported %v, PassAt %d", g.verbs.done, porter.Guests[0].PassAt)
	}
}

// The Fathers' memory lines, in a Room of the questline, from the 2nd time on.
func TestFathersRememberTheCircle(t *testing.T) {
	g, _ := verbRun(t, 2, 1)
	named(t, g, 0, "morwenna")
	k, m := &priest{}, &priest{strong: true}
	k.RoomIndex, m.RoomIndex = 0, 0
	if g.verbOpener(k) != nil || g.verbOpener(m) != nil {
		t.Error("memory lines the first time")
	}
	g.questRec.Times[0] = 1
	if l := g.verbOpener(k); len(l) != 1 || l[0] != fatherKQuest {
		t.Errorf("K: %v", l)
	}
	if l := g.verbOpener(m); l[0] != fatherMQuest[0] {
		t.Errorf("M: %v", l)
	}
	g.questRec.Times[0] = 2
	if l := g.verbOpener(m); l[0] != fatherMQuest[1] {
		t.Errorf("M the 3rd time: %v", l)
	}
	k.RoomIndex = 1
	if g.verbOpener(k) != nil {
		t.Error("a line in a Room with no quest guest")
	}
}

// O7: Magda moved into Osvaldo's Room (the override offers it) meets him.
func TestO7MagdaMeetsOsvaldo(t *testing.T) {
	g, _ := verbRun(t, 3, 5)
	m := named(t, g, 0, "magda")
	named(t, g, 2, "osvaldo")
	if q := g.questTarget(m); q != 2 {
		t.Fatalf("target %d, want Osvaldo's Room 2", q)
	}
	g.verbMinute()
	if reported(g, "O7") {
		t.Fatal("done apart")
	}
	g.relocate(m, 2)
	g.settleIn(m)
	g.verbMinute()
	if !reported(g, "O7") {
		t.Error("not done together")
	}
	if !strings.Contains(m.talk[len(m.talk)-1].text, "103") {
		t.Errorf("Magda's line: %v", m.talk)
	}
}

// O8: the Wreath row is on sale while O8 is open; bought, the Courier leaves it at Madame's door: a parcel in real,
// the wreath in ghost, where SPACE gives it.
func TestO8Wreath(t *testing.T) {
	g, s := verbRun(t, 2, 5)
	named(t, g, 1, "celestine")
	w := row(t, rowWreath)
	if !g.onSale(w) {
		t.Fatal("not on sale with O8 open")
	}
	g.quest.Done[1] = true
	if g.onSale(w) {
		t.Fatal("on sale with O8 done")
	}
	g.quest.Done[1] = false
	g.wallet[Money] = 500
	if why := g.addToCart(w, -1); why != "" {
		t.Fatal(why)
	}
	if why := g.addToCart(w, -1); why == "" {
		t.Error("two wreaths in the cart")
	}
	if why := g.placeOrder(); why != "" {
		t.Fatal(why)
	}
	if g.onSale(w) {
		t.Error("on sale while the courier carries it")
	}
	for i := 0; i < 4000 && g.rollGet("O8.parcel") == 0; i++ {
		g.step(0, 0)
	}
	if g.rollGet("O8.parcel") != 2 {
		t.Fatalf("parcel at %d, want Madame's Room (2)", g.rollGet("O8.parcel"))
	}
	if g.ghost || g.wreathAt(1) != true {
		t.Fatal("the parcel is not at her door")
	}
	press(g, s, center(g.hotel.Rooms[1].Mat())) // real: it is a parcel, SPACE does nothing quest
	if reported(g, "O8") {
		t.Fatal("given in real")
	}
	g.state = StatePlay
	g.ghost = true
	press(g, s, center(g.hotel.Rooms[1].Mat()))
	if !reported(g, "O8") || g.rollGet("O8.parcel") != 0 {
		t.Errorf("the wreath: reported %v parcel %d", g.verbs.done, g.rollGet("O8.parcel"))
	}
}

// O9: Ned's fact is where the key is; the oldest guest hands it over (TAKE), only that one; GIVE it to Ned in ghost.
func TestO9KeyForNed(t *testing.T) {
	g, s := verbRun(t, 3, 5)
	ned := named(t, g, 0, "ned")
	young, old := settledHuman(t, g, 1, 5), settledHuman(t, g, 2, 5)
	young.Guests[0].Who.Age, old.Guests[0].Who.Age = 30, 79
	g.verbMinute()
	if g.verbs.oldest != old || !g.verbPinned(old) || g.verbPinned(young) {
		t.Fatalf("oldest %v", g.verbs.oldest)
	}
	if hasOpt(optsOf(g, young), optTake) || !hasOpt(optsOf(g, old), optTake) {
		t.Error("TAKE should show on the oldest only")
	}
	g.startCard(2)
	if txt := flat(g); !strings.Contains(txt, keyGive) || !strings.Contains(txt, "TAKE THE KEY") {
		t.Errorf("the oldest's Card:\n%s", txt)
	}
	g.endTalk()
	g.cardVerb(optTake, old, old.Guests[0], &card{})
	if g.verbs.held != "O9" || g.verbPinned(old) || hasOpt(optsOf(g, old), optTake) {
		t.Errorf("after TAKE: held %q pinned %v", g.verbs.held, g.verbPinned(old))
	}
	g.ghost = true // SHIFT: Ned's Card
	ned.revealed = true
	g.startCard(0)
	if txt := flat(g); !strings.Contains(txt, "oldest soul") || !strings.Contains(txt, "GIVE THE KEY") {
		t.Errorf("Ned's Card:\n%s", txt)
	}
	os := g.cardOpts(ned, ned.Guests[0])
	tap(g, s, ebiten.KeyS)
	_ = os
	g.endTalk()
	g.cardVerb(optGive, ned, ned.Guests[0], &card{})
	if !reported(g, "O9") || g.verbs.held != "" {
		t.Errorf("GIVE: reported %v held %q", g.verbs.done, g.verbs.held)
	}
}

// O9 Lv2+: the oldest is a Tourist.
func TestO9OldestIsATouristAtLv2(t *testing.T) {
	g, _ := verbRun(t, 3, 5)
	g.level = 2
	old := settledHuman(t, g, 1, 5)
	old.Routine = homebody
	old.Guests[0].Who.Age = 80
	g.verbMinute()
	if old.Routine != tourist {
		t.Error("the oldest is not a Tourist")
	}
}

// O10: Morwenna into Madame's Room, both there at dawn.
func TestO10StrangeBedfellows(t *testing.T) {
	g, _ := verbRun(t, 3, 6)
	m := named(t, g, 0, "morwenna")
	named(t, g, 1, "celestine")
	if g.questTarget(m) != 1 {
		t.Fatal("no override toward Madame's Room")
	}
	g.relocate(m, 1)
	g.settleIn(m)
	minuteAt(g, 22*hourFrames)
	minuteAt(g, dayFrames+runStart)
	if !reported(g, "O10") {
		t.Error("not done at dawn")
	}
}

// O11: the rolled ghost ally hands the HAFT (TAKE, ghost); Morwenna takes it (GIVE, real). It crosses the SHIFT.
func TestO11Haft(t *testing.T) {
	g, _ := verbRun(t, 3, 6)
	m := named(t, g, 0, "morwenna")
	cel, ned := named(t, g, 1, "celestine"), named(t, g, 2, "ned")
	g.verbMinute()
	a := g.ally()
	if a != "celestine" && a != "ned" {
		t.Fatalf("ally %q", a)
	}
	giver, other := cel, ned
	if a == "ned" {
		giver, other = ned, cel
	}
	g.ghost = true
	if !hasOpt(optsOf(g, giver), optTake) || hasOpt(optsOf(g, other), optTake) {
		t.Fatal("TAKE should show on the rolled ally only")
	}
	g.cardVerb(optTake, giver, giver.Guests[0], &card{})
	if g.verbs.held != "O11" || hasOpt(optsOf(g, giver), optTake) {
		t.Fatalf("after TAKE: %q", g.verbs.held)
	}
	g.ghost = false
	if !hasOpt(optsOf(g, m), optGive) {
		t.Fatal("no GIVE on Morwenna's Card")
	}
	g.startCard(0)
	g.cardVerb(optGive, m, m.Guests[0], g.parley.card)
	if !reported(g, "O11") || !strings.Contains(flat(g), "A staff fit for a sorceress") {
		t.Errorf("GIVE: %v\n%s", g.verbs.done, flat(g))
	}
}

// O12: a rolled entry reads oddly in real and as the MacGuffin's name in ghost, where opening the page on it finishes O12.
func TestO12ReadTheName(t *testing.T) {
	g, _ := verbRun(t, 2, 6)
	for i := range 6 {
		g.guestBook = append(g.guestBook, bookEntry{"Guest", notebookCozy[i], 1, outCozy})
	}
	g.verbMinute()
	if !strings.Contains(entryLines(g), nameEntry) || strings.Contains(entryLines(g), macguffinName) {
		t.Fatalf("real: %s", entryLines(g))
	}
	g.openBook()
	if reported(g, "O12") {
		t.Fatal("read in real")
	}
	g.state, g.ghost = StatePlay, true
	if strings.Contains(entryLines(g), nameEntry) || !strings.Contains(entryLines(g), macguffinName) {
		t.Fatalf("ghost: %s", entryLines(g))
	}
	g.openBook()
	for i := 0; i < 5 && !reported(g, "O12"); i++ { // the entry may be on a later spread
		g.in.(*script).hold(ebiten.KeyD)
		g.updateBook()
	}
	if !reported(g, "O12") {
		t.Error("the name was never read")
	}
}

// O13: the Crate row is on sale while O13 is open; the Courier carries it to the Entrance Mirror.
func TestO13Crate(t *testing.T) {
	g, _ := verbRun(t, 2, 7)
	g.own(Mirror)
	c := row(t, rowCrate)
	if !g.onSale(c) {
		t.Fatal("not on sale with O13 open")
	}
	g.wallet = [2]Num{5000, 1000}
	if why := g.addToCart(c, -1); why != "" {
		t.Fatal(why)
	}
	if why := g.placeOrder(); why != "" {
		t.Fatal(why)
	}
	for i := 0; i < 4000 && !reported(g, "O13"); i++ {
		g.step(0, 0)
	}
	if !reported(g, "O13") {
		t.Error("the crate never reached the Mirror")
	}
	g.quest.Done[0] = true
	if g.onSale(c) {
		t.Error("on sale with O13 done")
	}
}

// A guest with an Ask says it instead of its line, while the objective is open and no other.
func TestAskReplacesLine(t *testing.T) {
	g, _ := verbRun(t, 2, 3)
	named(t, g, 0, "morwenna")
	g.startCard(0)
	if txt := flat(g); !strings.Contains(txt, "customer service of the dead") {
		t.Errorf("O3's Ask:\n%s", txt)
	}
	g.endTalk()
	g.quest.Done[0] = true
	g.startCard(0)
	if txt := flat(g); strings.Contains(txt, "customer service") {
		t.Errorf("the Ask stays after O3:\n%s", txt)
	}
}

// Nothing of a quest happens with no objective open.
func TestVerbsInertWithoutQuest(t *testing.T) {
	g, _ := matchRun(t, 3)
	m := named(t, g, 0, "morwenna")
	named(t, g, 1, "celestine")
	g.verbMinute()
	g.updateVerbs()
	if g.questTarget(m) >= 0 || len(g.verbActors()) != 0 || g.verbPinned(m) || len(g.questEntries(nil)) != 0 {
		t.Error("the verbs act with no quest")
	}
	if on, quest := g.questRowOn(rowWreath); on || !quest {
		t.Error("the wreath row")
	}
	_ = world.TileSize
	_ = image.Point{}
}
