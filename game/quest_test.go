package game

import (
	"slices"
	"testing"
)

// questRun0: a started Run, every Room owned, the Mirror open (ghosts book), at prestige level lv.
func questRun0(t *testing.T, lv int) *Game {
	t.Helper()
	g, _ := headless(t, 3, 3, 1)
	g.level = lv
	g.startRun()
	g.unlock(Mirror)
	return g
}

// finish ticks every objective of the beat under way, by ID.
func finish(g *Game) {
	for _, o := range beats[g.quest.Beat-1].Objs {
		g.questEvent(evDone, -1, o.ID)
	}
}

// grow starts the next beat and checks its guests in.
func grow(g *Game) {
	g.questEvent(evGrow, -1, "")
	for range 3 { // a guest a minute, each settled in before the next
		g.questArrivals()
		for _, p := range g.parties {
			g.settleIn(p)
		}
	}
}

// W5-QUEST: a beat starts on each Growth, as many a Run as its level plays, its Done sized to its objectives.
func TestQuestBeatsStartOnGrow(t *testing.T) {
	for lv, want := range []int{3, 5, 7, 7} {
		g := questRun0(t, lv)
		if g.quest.Beat != 0 || g.quest.Rolls == nil || g.questStage() != 0 {
			t.Fatalf("level %d: a new Run is at beat %d, Rolls %v", lv, g.quest.Beat, g.quest.Rolls)
		}
		for k := 1; k <= want+2; k++ {
			g.questEvent(evGrow, -1, "")
			b := min(k, want)
			if g.quest.Beat != b || len(g.quest.Done) != len(beats[b-1].Objs) || slices.Contains(g.quest.Done, true) {
				t.Fatalf("level %d, Growth %d: beat %d, Done %v", lv, k, g.quest.Beat, g.quest.Done)
			}
			if k < want {
				finish(g) // the gate would stop the next Growth otherwise
			}
		}
	}
}

// Objectives tick by ID, in any order; the beat is done with the last one, and counts once.
func TestQuestTickByID(t *testing.T) {
	g := questRun0(t, 1)
	for range 5 {
		g.questEvent(evGrow, -1, "")
		if g.quest.Beat < 5 {
			finish(g)
		}
	}
	g.questEvent(evDone, -1, "O1")  // not of this beat
	g.questEvent(evScene, -1, "O9") // a cutscene's end: information, it ticks nothing
	if g.quest.Done[2] {
		t.Fatal("the O9 cutscene's end ticked O9")
	}
	g.questEvent(evTell, 3, "O9")
	if !g.quest.Done[2] || g.quest.Done[0] || g.questOpen("O9") || !g.questOpen("O7") {
		t.Fatalf("Done %v after O9", g.quest.Done)
	}
	g.questEvent(evGive, 1, "O7")
	if g.timesDone(5) != 0 || g.questStage() != 4 {
		t.Fatal("beat 5 done with an objective open")
	}
	g.questEvent(evServe, 1, "O8")
	g.questEvent(evServe, 1, "O8") // ticked twice: no second count
	if g.timesDone(5) != 1 || g.questStage() != 5 {
		t.Fatalf("beat 5 done %d times, stage %d", g.timesDone(5), g.questStage())
	}
}

// The first time ever a beat is done it pays its purse and gives its item; a replay in a new Run pays nothing and
// the item stays.
func TestQuestRewardOnceEver(t *testing.T) {
	g := questRun0(t, 0)
	g.questEvent(evGrow, -1, "")
	ecto := g.wallet[Ecto]
	finish(g)
	if g.wallet[Ecto]-ecto != beats[0].Purse*Num(g.bonus) || !g.itemOn(itemBall) || g.itemOn(itemBroom) {
		t.Fatalf("Ecto +%v, ball %v: want the purse and the ball only", g.wallet[Ecto]-ecto, g.itemOn(itemBall))
	}
	if len(g.note.items) != 2 {
		t.Fatalf("%d notices, want BEAT DONE and the item", len(g.note.items))
	}
	g.startRun()
	g.questEvent(evGrow, -1, "")
	ecto = g.wallet[Ecto]
	finish(g)
	if g.wallet[Ecto] != ecto || g.timesDone(1) != 2 || !g.itemOn(itemBall) {
		t.Fatalf("a replay: Ecto %v -> %v, done %d times, ball %v", ecto, g.wallet[Ecto], g.timesDone(1), g.itemOn(itemBall))
	}
	g.quest.Beat, g.quest.Done = 7, make([]bool, 1) // the last beat pays its purse (0) and gives no item
	g.questEvent(evDone, -1, "O13")
	if g.timesDone(7) != 1 || g.itemOn(itemBroom) {
		t.Fatalf("beat 7: done %d, items %v", g.timesDone(7), g.questRec.Items)
	}
}

// The next Growth waits for the beat's objectives, but not on the Run's last beat.
func TestQuestGate(t *testing.T) {
	g, _ := headless(t, 2, 1, 1)
	g.startRun()
	if g.questGate() != "" {
		t.Fatal("gated before beat 1")
	}
	g.questEvent(evGrow, -1, "")
	if g.questGate() != gateRefuse || g.growRefuse(growCol) != gateRefuse {
		t.Fatalf("gate %q with O1 open", g.questGate())
	}
	finish(g)
	if g.questGate() != "" {
		t.Fatal("gated with the beat done")
	}
	g.questEvent(evGrow, -1, "")
	finish(g)
	g.questEvent(evGrow, -1, "") // beat 3, the Run's last at level 0: nothing left to wait for
	if g.questGate() != "" {
		t.Fatal("gated on the Run's last beat")
	}
}

// A quest guest is pinned while an objective needs it: it does not check out. A plain guest is not pinned.
func TestQuestPinned(t *testing.T) {
	g := questRun0(t, 1)
	grow(g)
	p, _ := g.questHolder("morwenna")
	if p == nil || !g.questPinned(p) {
		t.Fatal("Morwenna is not in, or not pinned, on beat 1")
	}
	g.settleIn(p)
	g.clock = p.Checkout + 1
	g.updateParty(p)
	if p.state == checkingOut {
		t.Fatal("a pinned guest checked out")
	}
	if other := g.walkIn(); other == nil || g.questPinned(other) {
		t.Fatal("a plain guest is pinned")
	}
	finish(g)
	for range 4 {
		grow(g)
		finish(g)
	}
	for _, key := range []string{"morwenna", "celestine", "magda", "ned", "teo", "osvaldo"} { // level 1: five beats, all done
		if g.questNeeds(key) {
			t.Errorf("%s still needed after the Run's beats", key)
		}
	}
	g.updateParty(p)
	if p.state != checkingOut {
		t.Fatal("Morwenna stays after the Run's last beat")
	}
}

// Each guest is needed through its own objective: Magda and Osvaldo through O7, Ned to the finale, Teo only to O6.
func TestQuestNeedsThrough(t *testing.T) {
	g := questRun0(t, 2)
	for range 4 {
		g.questEvent(evGrow, -1, "")
		finish(g)
	}
	g.questEvent(evGrow, -1, "") // beat 5
	if !g.questNeeds("magda") || !g.questNeeds("osvaldo") || !g.questNeeds("morwenna") || g.questNeeds("teo") {
		t.Fatal("beat 5: Magda, Osvaldo and Morwenna are needed, Teo is not")
	}
	g.questEvent(evDone, -1, "O7")
	if g.questNeeds("magda") || g.questNeeds("osvaldo") || !g.questNeeds("ned") {
		t.Fatal("after O7 Magda and Osvaldo go; Ned stays for the finale")
	}
}

// A beat's guests check in ahead of the walk-ins, once, in the right Dimension, named and counted as visits.
func TestQuestArrivals(t *testing.T) {
	g := questRun0(t, 0)
	g.questEvent(evGrow, -1, "")
	g.clock += minuteFrames - g.clock%minuteFrames - 1
	g.advanceClock() // questArrivals runs before maybeWalkIn
	if len(g.parties) == 0 || g.parties[0].Guests[0].Who.Key != "morwenna" || g.parties[0].Realm != Human || g.visits["morwenna"] != 1 || !g.met["morwenna"] {
		t.Fatalf("%d parties: want Morwenna first, a visit", len(g.parties))
	}
	n := len(g.parties)
	g.questArrivals()
	if len(g.parties) != n {
		t.Fatal("Morwenna checked in twice")
	}
	finish(g)
	grow(g) // beat 2: Celestine, a ghost
	p, _ := g.questHolder("celestine")
	if p == nil || p.Realm != Ghost || g.visits["celestine"] != 1 || p.Guests[0].Who.Name != "Madame Celestine Arbogast" {
		t.Fatalf("Celestine: %+v", p)
	}
	if m, _ := g.questHolder("morwenna"); m == nil {
		t.Fatal("Morwenna left")
	}
	finish(g)
	grow(g) // beat 3: Magda
	if m, _ := g.questHolder("magda"); m == nil || m.Realm != Human {
		t.Fatal("Magda is not in")
	}
}

// A guest lost to a resumed Run (no Party survives a save) checks in again; none is booked with no Room free.
func TestQuestArrivalsAgain(t *testing.T) {
	g := questRun0(t, 0)
	g.questEvent(evGrow, -1, "")
	g.roomOwned = make([]bool, len(g.hotel.Rooms)) // nothing to book into
	g.questArrivals()
	if p, _ := g.questHolder("morwenna"); p != nil {
		t.Fatal("booked with no Room owned")
	}
	g.roomOwned = nil
	g.questArrivals()
	if p, _ := g.questHolder("morwenna"); p == nil {
		t.Fatal("did not check in once a Room was free")
	}
	g.parties = nil
	g.questArrivals()
	if p, _ := g.questHolder("morwenna"); p == nil {
		t.Fatal("did not come back")
	}
}

// The beat's opening line is said by its Speaker once it is in; a beat's cliffhanger when it ends.
func TestQuestLines(t *testing.T) {
	g := questRun0(t, 0)
	grow(g)
	p, who := g.questHolder("morwenna")
	if len(p.talk) == 0 || p.talk[0].who != who || !g.quest.Said {
		t.Fatalf("no opening line: %v", p.talk)
	}
	for _, l := range p.talk {
		if len(l.text) > sayWrap {
			t.Errorf("%q does not fit a balloon", l.text)
		}
	}
	finish(g)
	grow(g)
	finish(g)
	grow(g)
	p.talk = nil
	finish(g) // beat 3: Morwenna's cliffhanger, at its end
	if len(p.talk) == 0 {
		t.Fatal("no cliffhanger")
	}
}

// A Run resumed from a save with a beat under way keeps playing it: the beat, its ticks, its Rolls and the record.
func TestQuestSaveRoundTrip(t *testing.T) {
	g1 := questRun0(t, 0)
	g1.questEvent(evGrow, -1, "")
	finish(g1)
	g1.questEvent(evGrow, -1, "")
	finish(g1)
	g1.questEvent(evGrow, -1, "") // beat 3
	g1.quest.Rolls["O3.room"] = 4
	g1.questEvent(evDone, -1, "O4")
	data, _, _ := memStore(t)
	g1.persist = true
	g1.saveDawn()
	s := g1.loadSave()
	if s == nil || s.Run == nil || s.Quest == nil {
		t.Fatalf("saved %s", *data)
	}
	g2, _ := headless(t, 3, 3, 2)
	g2.startRun()
	g2.persist = true
	if !g2.resume(s) {
		t.Fatal("did not resume")
	}
	if g2.quest.Beat != 3 || !g2.quest.Done[1] || g2.quest.Done[0] || g2.quest.Rolls["O3.room"] != 4 || g2.timesDone(1) != 1 || !g2.itemOn(itemBall) {
		t.Fatalf("resumed %+v, record %+v", g2.quest, g2.questRec)
	}
	if !g2.questOpen("O3") || g2.questOpen("O4") || g2.questStage() != 2 {
		t.Fatal("the beat under way is not playable")
	}
	g2.unlock(Mirror)
	g2.questArrivals() // Morwenna is back
	if p, _ := g2.questHolder("morwenna"); p == nil {
		t.Fatal("Morwenna did not come back")
	}
	g2.questEvent(evTell, -1, "O3")
	if g2.timesDone(3) != 1 {
		t.Fatal("the resumed beat did not finish")
	}
}

// The quester completes objectives: a headless Run of a few days finishes beat 1 with no failure.
func TestQuesterAdvances(t *testing.T) {
	s, err := newSim(Config{Cols: 2, Rows: 1, Days: 8, Seed: 1, Name: "quester", Strategy: Strategies["quester"], React: 30, KeyGap: 6})
	if err != nil {
		t.Fatal(err)
	}
	for !s.done() && s.frame() {
	}
	if r := s.finish(); len(r.Failures) > 0 || !r.Cheat {
		t.Fatalf("failures %v, cheat %v", r.Failures, r.Cheat)
	}
	if s.g.quest.Beat < 1 || s.g.timesDone(1) < 1 {
		t.Fatalf("beat %d, beat 1 done %d times: the questline never moved", s.g.quest.Beat, s.g.timesDone(1))
	}
}
