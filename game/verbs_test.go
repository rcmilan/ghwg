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

// verbsOf: the quest verbs that apply to the Party's first guest (quest TALK does them).
func verbsOf(g *Game, p *Party) []cardOpt { return g.verbOpts(p, p.Guests[0]) }

// talkTo: SPACE with the Concierge on the Party's first guest; what it says over its head after, wraps undone.
func talkTo(g *Game, s *script, p *Party) string {
	guest := p.Guests[0]
	p.talk = nil
	press(g, s, image.Pt(int(guest.X), int(guest.Y)))
	var ls []string
	for _, l := range p.talk {
		ls = append(ls, l.text)
	}
	return strings.Join(append(ls, talkOut(g)...), " ")
}

// talkOut plays a quest TALK's close-in to its end, returning the lines said in it.
func talkOut(g *Game) (ls []string) {
	for i := 0; g.scene.id == "talk"; i++ {
		if l := g.scene.line; l != "" && !slices.Contains(ls, l) {
			ls = append(ls, l)
		}
		if i > 10000 {
			panic("the talk never ends")
		}
		g.stepScene()
	}
	return ls
}

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

// O1: talking to Magenta in real gives the word; it crosses the SHIFT, and one SPACE on the ghost roommate, in ghost
// only, tells it.
func TestO1TellAcrossShift(t *testing.T) {
	g, s := verbRun(t, 2, 1)
	m := named(t, g, 0, "magenta")
	if g.verbWait("O1") != true {
		t.Error("no ghost in her Room, but the row does not wait")
	}
	mate := ghostOf(g, 0, 3)
	if g.verbWait("O1") {
		t.Error("a ghost is in her Room, and the row still waits")
	}
	if hasOpt(verbsOf(g, m), optTell) {
		t.Error("TELL applies with no word held")
	}
	if txt := talkTo(g, s, m); g.state != StatePlay || g.verbs.held != "O1" || !strings.Contains(txt, "witch has arrived") {
		t.Fatalf("her talk: state %d, held %q, said %q; want the word held and the Ask said", g.state, g.verbs.held, txt)
	}
	if g.tick >= m.hold {
		t.Error("she did not stop")
	}
	if q, _ := g.questTalk(); q == mate {
		t.Error("real: SPACE talks to the ghost")
	}
	g.ghost = true // SHIFT: the word stays
	if g.verbs.held != "O1" {
		t.Fatal("the word did not cross the SHIFT")
	}
	if txt := talkTo(g, s, mate); !reported(g, "O1") || g.verbs.held != "" || !strings.Contains(txt, "A witch! Finally") {
		t.Errorf("one SPACE: reported %v, held %q, said %q", g.verbs.done, g.verbs.held, txt)
	}
}

// A quest character walking in the Corridor can be talked to, and stops for it; inside the Room SPACE talks before
// it cleans; with nothing quest-related it says one of its lines.
func TestQuestTalkWalkingAndBeforeClean(t *testing.T) {
	g, s := verbRun(t, 2, 1)
	m := named(t, g, 0, "magenta")
	g.state = StatePlay
	g.verbs.held = ""
	x, y := g.hotel.Rooms[0].Door.X*world.TileSize+8, g.hotel.Rooms[0].Door.Y*world.TileSize+3*world.TileSize
	m.Guests[0].X, m.Guests[0].Y = float64(x), float64(y)
	m.state, m.Guests[0].Path = checkingIn, []image.Point{{x, y + 64}}
	g.player.X, g.player.Y = float64(x), float64(y)
	if act, _ := g.actionable(); act != actQuestTalk {
		t.Fatalf("walking, close: SPACE is %d, want quest TALK", act)
	}
	g.player.X = float64(x + 3*world.TileSize)
	if act, _ := g.actionable(); act == actQuestTalk {
		t.Error("far from her: still quest TALK")
	}
	m.state, m.Guests[0].Path = staying, nil
	g.settleIn(m)
	g.questSpeak(m, m.Guests[0])
	if g.scene.id != "talk" || g.verbs.held != "" {
		t.Error("the talk hands the word over before the close-in and her line")
	}
	talkOut(g)
	if g.verbs.held != "O1" || g.tick >= m.hold {
		t.Errorf("walking talk: held %q", g.verbs.held)
	}
	at := m.Guests[0]
	g.player.X, g.player.Y = at.X, at.Y
	if act, _ := g.actionable(); act == actQuestTalk {
		t.Error("SPACE talks again to her while she is stopped")
	}
	m.hold = 0
	g.setDirtLevel(0, spotUnit) // something to clean in her Room
	if room, _ := g.cleanable(); room != 0 {
		t.Fatal("nothing to clean in her Room")
	}
	if act, _ := g.actionable(); act != actQuestTalk {
		t.Errorf("inside the Room: SPACE is %d, want quest TALK before clean", act)
	}
	m.hold = 0
	m.Guests[0].Who.Lines = []string{"Hmm."} // copy: placeholder line for the test
	g.quest.Done[0], g.verbs.held, g.verbs.beat = true, "", g.quest.Beat
	if txt := talkTo(g, s, m); txt == "" {
		t.Error("nothing quest-related: she says nothing")
	}
}

// The Card has no quest verbs: TELL, TAKE and GIVE are quest TALK's.
func TestCardHasNoQuestVerbs(t *testing.T) {
	g, _ := verbRun(t, 3, 1)
	m, ghost := named(t, g, 0, "magenta"), ghostOf(g, 0, 3)
	for _, held := range []string{"O1", "O2", "O9", "O11"} {
		g.verbs.held = held
		g.ghost = false
		for _, p := range []*Party{m, ghost} {
			g.ghost = p.Realm == Ghost
			for _, o := range optsOf(g, p) {
				if o >= optTell {
					t.Errorf("held %q: a quest option %d on %v's Card", held, o, p.Guests[0].Who.Name)
				}
			}
		}
	}
}

// Quest TALK does only what applies: no TELL, TAKE or GIVE without a word or thing for that guest, none for a closed objective.
func TestVerbMenusOnlyWhatApplies(t *testing.T) {
	g, _ := verbRun(t, 3, 1)
	m, ghost := named(t, g, 0, "magenta"), ghostOf(g, 0, 3)
	for _, held := range []string{"", "O2", "O9", "O11"} {
		g.verbs.held = held
		for _, p := range []*Party{m, ghost} {
			if os := verbsOf(g, p); len(os) > 0 {
				t.Errorf("held %q: quest verbs %v on %v", held, os, p.Guests[0].Who.Name)
			}
		}
	}
	g.verbs.held = "O1"
	if !hasOpt(verbsOf(g, ghost), optTell) || hasOpt(verbsOf(g, m), optTell) {
		t.Error("held O1: TELL should apply to the ghost only")
	}
	g.quest.Done[0] = true
	if hasOpt(verbsOf(g, ghost), optTell) {
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

// O2: Celestine's entry is in the book only in ghost; reading it hands the word; TELL Magenta in real. Lv2+ needs the
// second part too.
func TestO2BookEntryGhostOnly(t *testing.T) {
	for _, level := range []int{0, 2} {
		g, s := verbRun(t, 2, 2)
		g.level = level
		setQuest(g, 2)
		named(t, g, 1, "celestine")
		m := named(t, g, 0, "magenta")
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
		if !hasOpt(verbsOf(g, m), optTell) {
			t.Fatal("no TELL for Magenta")
		}
		if txt := talkTo(g, s, m); !reported(g, "O2") || !strings.Contains(txt, "Ninety-seven years") {
			t.Errorf("level %d: reported %v: %q", level, g.verbs.done, txt)
		}
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
	if to, _, _, ok := g.sureOffer(m); ok { // the housekeepers keep off the quest (R10)
		t.Errorf("sureOffer %d for a quest Party", to)
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
	if g.scene.id != "O4" || !g.verbs.spirit.on {
		t.Fatal("the O4 scene should be playing with the spirit still standing in the fade")
	}
	for g.scene.t <= o4Fade { // the spirit is gone once the screen is black
		g.stepScene()
	}
	if g.verbs.spirit.on || g.verbs.spirit.fold != 0 {
		t.Error("the spirit is still there in the black")
	}
	for g.scene.on() {
		g.stepScene()
	}
	g.ghost = true // SHIFT: SPACE on the door
	s := &script{}
	g.in = s
	press(g, s, center(g.hotel.Rooms[1].Mat()))
	if g.scene.id != "hand" || reported(g, "O4") {
		t.Fatalf("SPACE on the handprint door should start its close-up first (scene %q)", g.scene.id)
	}
	for i := 0; g.scene.on() && i < 1000; i++ {
		g.stepScene()
	}
	if !reported(g, "O4") {
		t.Error("the handprint close-up did not finish O4")
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

// O5: only a quest ghost asks; Tim asks for a Supply, fetched in real and given at his door in ghost.
func TestO5GhostRequest(t *testing.T) {
	g, s := verbRun(t, 2, 4)
	tim := named(t, g, 0, "tim")
	other := ghostOf(g, 1, 3)
	for range 3 * dayFrames / minuteFrames {
		g.clock += minuteFrames
		g.updateAsks()
	}
	if other.asking() || tim.asking() {
		t.Fatal("a ghost asked through the ordinary asks")
	}
	g.clock = runStart
	minuteAt(g, runStart)
	if !tim.asking() || other.asking() {
		t.Fatalf("Tim asks %v, another ghost %v", tim.asking(), other.asking())
	}
	want := tim.want
	g.ghost, g.hands = true, SupplyFood
	if want == SupplyFood {
		g.hands = SupplyWater
	}
	press(g, s, center(g.hotel.Rooms[0].Mat()))
	if reported(g, "O5") || !tim.asking() {
		t.Fatal("the wrong Supply was taken")
	}
	g.state = StatePlay // SPACE with nothing to do opened the hotel view
	g.hands = want
	press(g, s, center(g.hotel.Rooms[0].Mat()))
	if !reported(g, "O5") || tim.asking() || g.hands != "" {
		t.Errorf("delivered: reported %v asking %v hands %q", g.verbs.done, tim.asking(), g.hands)
	}
}

func TestO5TwoAsksAtLv2(t *testing.T) {
	g, s := verbRun(t, 2, 4)
	g.level = 2
	tim := named(t, g, 0, "tim")
	g.ghost = true
	minuteAt(g, runStart)
	first := tim.want
	g.hands = first
	press(g, s, center(g.hotel.Rooms[0].Mat()))
	if reported(g, "O5") {
		t.Fatal("done after one of two")
	}
	g.state = StatePlay
	minuteAt(g, runStart+minuteFrames)
	if !tim.asking() || tim.want == first {
		t.Fatalf("no second, different ask: asking %v %q after %q", tim.asking(), tim.want, first)
	}
	g.hands = tim.want
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
	pr := &priest{strong: true}
	pr.RoomIndex = 1
	if l := g.verbOpener(pr); len(l) != 1 || l[0] != fatherMParty[0] {
		t.Errorf("M's opener: %v", l)
	}
	g.questRec.Times[3] = 1
	if l := g.verbOpener(pr); l[0] != fatherMParty[1] {
		t.Errorf("M's opener the 2nd time: %v", l)
	}
	g.ritualEffect(&priest{Guest: entities.Guest{RoomIndex: 1}}) // K's Ritual: no party
	if reported(g, "O6") {
		t.Fatal("Father K finished O6")
	}
	g.ritualEffect(pr)
	if !reported(g, "O6") || porter.Guests[0].PassAt == 0 {
		t.Errorf("M's Ritual: reported %v, PassAt %d", g.verbs.done, porter.Guests[0].PassAt)
	}
}

// The Fathers' memory lines, in a Room of the questline, from the 2nd time on.
func TestFathersRememberTheCircle(t *testing.T) {
	g, _ := verbRun(t, 2, 1)
	named(t, g, 0, "magenta")
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

// O9: Tim's fact is where the key is; the oldest guest hands it over (TAKE), only that one; GIVE it to Tim in ghost.
func TestO9KeyForTim(t *testing.T) {
	g, s := verbRun(t, 3, 5)
	tim := named(t, g, 0, "tim")
	young, old := settledHuman(t, g, 1, 5), settledHuman(t, g, 2, 5)
	young.Guests[0].Who.Age, old.Guests[0].Who.Age = 30, 79
	g.verbMinute()
	if g.verbs.oldest != old || !g.verbPinned(old) || g.verbPinned(young) {
		t.Fatalf("oldest %v", g.verbs.oldest)
	}
	if hasOpt(verbsOf(g, young), optTake) || !hasOpt(verbsOf(g, old), optTake) {
		t.Error("TAKE should apply to the oldest only")
	}
	if txt := talkTo(g, s, old); !strings.Contains(txt, keyGive) || g.verbs.held != "O9" || g.verbPinned(old) || hasOpt(verbsOf(g, old), optTake) {
		t.Errorf("after TALK: said %q, held %q pinned %v", txt, g.verbs.held, g.verbPinned(old))
	}
	g.ghost = true // SHIFT: Tim
	talkTo(g, s, tim)
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

// A quest move offers only its Room: with other Rooms to go to, the talk lists Madame's alone (O10).
func TestQuestMoveOffersOnlyThatRoom(t *testing.T) {
	g, _ := verbRun(t, 3, 6)
	m := named(t, g, 0, "magenta")
	named(t, g, 1, "celestine")
	settledHuman(t, g, 2, 12) // somewhere else it could go, were it free
	if ts := g.targets(m); len(ts) != 1 || ts[0] != 1 {
		t.Errorf("targets %v, want only Madame's Room 1", ts)
	}
}

// O10: Magenta into Madame's Room, both there at dawn.
func TestO10StrangeBedfellows(t *testing.T) {
	g, _ := verbRun(t, 3, 6)
	m := named(t, g, 0, "magenta")
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

// O11: the rolled ghost ally hands the HAFT (TAKE, ghost); Magenta takes it (GIVE, real). It crosses the SHIFT.
func TestO11Haft(t *testing.T) {
	g, s := verbRun(t, 3, 6)
	m := named(t, g, 0, "magenta")
	cel, tim := named(t, g, 1, "celestine"), named(t, g, 2, "tim")
	g.verbMinute()
	a := g.ally()
	if a != "celestine" && a != "tim" {
		t.Fatalf("ally %q", a)
	}
	giver, other := cel, tim
	if a == "tim" {
		giver, other = tim, cel
	}
	g.ghost = true
	if !hasOpt(verbsOf(g, giver), optTake) || hasOpt(verbsOf(g, other), optTake) {
		t.Fatal("TAKE should apply to the rolled ally only")
	}
	talkTo(g, s, giver)
	if g.verbs.held != "O11" || hasOpt(verbsOf(g, giver), optTake) {
		t.Fatalf("after TALK: %q", g.verbs.held)
	}
	g.ghost = false
	if !hasOpt(verbsOf(g, m), optGive) {
		t.Fatal("no GIVE for Magenta")
	}
	if txt := talkTo(g, s, m); !reported(g, "O11") || !strings.Contains(txt, "Fits my staff like it grew there") {
		t.Errorf("GIVE: %v: %q", g.verbs.done, txt)
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

// O13: the Crate row is on sale while O13 is open; the Courier carries it into Madame's Room.
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
	shell := g.hotel.Rooms[max(0, g.roomOfKey("celestine"))].Shell()
	in := false
	for _, d := range g.couriers[0].drops {
		in = in || len(d.lines) > 0 && d.at.In(shell)
	}
	if !in {
		t.Error("the crate's drop is not inside Madame's Room")
	}
	for i := 0; i < 4000 && !reported(g, "O13"); i++ {
		g.step(0, 0)
	}
	if !reported(g, "O13") {
		t.Error("the crate never reached Madame's Room")
	}
	g.quest.Done[0] = true
	if g.onSale(c) {
		t.Error("on sale with O13 done")
	}
}

// A guest with an Ask says it instead of its line, while the objective is open and no other.
func TestAskReplacesLine(t *testing.T) {
	g, _ := verbRun(t, 2, 3)
	named(t, g, 0, "magenta")
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
	m := named(t, g, 0, "magenta")
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

// O10: Madame's Room is reserved for Magenta (no walk-in books it) and stays offered though Closed.
func TestO10RoomReserved(t *testing.T) {
	g, _ := verbRun(t, 3, 6)
	m := named(t, g, 0, "magenta")
	named(t, g, 1, "celestine")
	g.addMood(1, -1000)
	if !g.closed(1) || !g.reserved(1) {
		t.Fatal("Room 1 should be closed and reserved")
	}
	if !slices.Contains(g.targets(m), 1) {
		t.Error("closed Room dropped from the talk")
	}
	if g.freeRoom(1) == 1 || g.fitRoom(nerveMin, 1) == 1 {
		t.Error("a walk-in could book the reserved Room")
	}
	g.relocate(m, 1)
	if g.reserved(1) || g.questRoom(1) != true {
		t.Error("Magenta in: no longer reserved, but still no Priest")
	}
}

// O10 soft lock (live test 3): with Matchmaking and an idle housekeeper, Magenta moved into Madame's Room (a bad pair
// by Nerve) is not sent back out; she is still there at dawn and the objective completes.
func TestO10MatchmakingKeepsOff(t *testing.T) {
	for _, mm := range []bool{false, true} {
		g, _ := verbRun(t, 3, 6)
		g.wallet[Money] = 10 * keeperWage
		g.hire()
		if mm {
			g.own(Matchmaking)
		}
		m := named(t, g, 0, "magenta")
		cel := named(t, g, 1, "celestine")
		setNerve(m, 2)
		cel.Guests[0].Phantasm = 9
		settledHuman(t, g, 2, 12) // a free-looking Room 2 to run to
		g.relocate(m, 1)
		g.clock = 20 * hourFrames
		for f := 0; g.clock < dayFrames+runStart+hourFrames && f < 3*dayFrames; f++ {
			g.step(0, 0)
			if f%50 == 0 && m.Room != 1 {
				t.Fatalf("matchmaking %v: Magenta sent to Room %d at clock %d", mm, m.Room, g.clock)
			}
		}
		if !reported(g, "O10") {
			t.Errorf("matchmaking %v: not done at dawn (room %d state %v clock %d)", mm, m.Room, m.state, g.clock)
		}
	}
}

// O9: talking to Tim in ghost says the objective's Ask, and that ticks "ASK TIM ABOUT THE KEY".
func TestO9TalkAsksTim(t *testing.T) {
	g, s := verbRun(t, 2, 5)
	tim := named(t, g, 0, "tim")
	g.ghost = true
	standIn(g, 0)
	if g.sawn("O9.read") {
		t.Fatal("seen before the talk")
	}
	if said := talkTo(g, s, tim); !strings.Contains(strings.Join(strings.Fields(said), " "), strings.Join(strings.Fields(objOf("O9").Ask), " ")) || !g.sawn("O9.read") {
		t.Errorf("Tim said %q; sawn %v", said, g.sawn("O9.read"))
	}
}
