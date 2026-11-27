package game

import (
	"image"
	"math"
	"slices"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
)

// 08 W5 R1 GUIDE: the checklist's next step per objective, from the quest and verb state.
func TestNextStep(t *testing.T) {
	type want struct {
		dim  Realm
		any  bool
		kind tgKind
		key  string
	}
	cases := []struct {
		name  string
		obj   string
		level int
		set   func(g *Game)
		want  want
	}{
		{"O1 start", "O1", 0, func(g *Game) { named(t, g, 0, "magenta") }, want{Human, false, tgGuest, "magenta"}},
		{"O1 word, no roommate", "O1", 0, func(g *Game) { named(t, g, 0, "magenta"); g.verbs.held = "O1" }, want{Ghost, false, tgNone, ""}},
		{"O1 word, roommate", "O1", 0, func(g *Game) {
			named(t, g, 0, "magenta")
			g.placeNamed(0, "celestine")
			g.verbs.held = "O1"
		}, want{Ghost, false, tgGuest, "ghostmate"}},
		{"O1 missed dawn", "O1", 2, func(g *Game) { named(t, g, 0, "magenta"); g.rollSet("O1.long", 1) }, want{Human, false, tgGuest, "magenta"}},
		{"O2 read", "O2", 0, func(g *Game) {}, want{Ghost, false, tgBook, ""}},
		{"O2 half read at Lv2", "O2", 2, func(g *Game) { g.rollSet("O2.read", 1) }, want{Ghost, false, tgBook, ""}},
		{"O2 tell", "O2", 0, func(g *Game) { g.verbs.held = "O2" }, want{Human, false, tgGuest, "magenta"}},
		{"O3 find her room", "O3", 0, func(g *Game) {}, want{Ghost, false, tgGuest, "celestine"}},
		{"O3 move Magda", "O3", 0, func(g *Game) { g.saw("celestine") }, want{Human, false, tgGuest, "magda"}},
		{"O3 stranger", "O3", 2, func(g *Game) {
			g.saw("celestine")
			c := named(t, g, 1, "celestine")
			g.verbs.stranger = g.arrive(0)
			g.verbs.stranger.Room = c.Room
			named(t, g, 2, "magda")
		}, want{Human, false, tgGuest, "stranger"}},
		{"O3 wait for dawn", "O3", 0, func(g *Game) {
			g.saw("celestine")
			named(t, g, 1, "celestine")
			named(t, g, 1, "magda")
		}, want{Human, false, tgNone, ""}},
		{"O4 day", "O4", 0, func(g *Game) { g.clock = 12 * hourFrames }, want{Human, false, tgNone, ""}},
		{"O4 night", "O4", 0, func(g *Game) { g.clock = 23 * hourFrames }, want{Human, false, tgSpirit, ""}},
		{"O4 lit", "O4", 0, func(g *Game) { g.rollSet("O4.lit", 1) }, want{Ghost, false, tgDoor, "handprint"}},
		{"O5 no ask", "O5", 0, func(g *Game) { named(t, g, 0, "tim") }, want{Ghost, false, tgGuest, "tim"}},
		{"O5 fetch", "O5", 0, func(g *Game) {
			p := named(t, g, 0, "tim")
			p.asked, p.want = 1, SupplyTowel
			g.saw("O5.ask")
		}, want{Human, false, tgSupply, ""}},
		{"O5 bring", "O5", 0, func(g *Game) {
			p := named(t, g, 0, "tim")
			p.asked, p.want, g.hands = 1, SupplyTowel, SupplyTowel
		}, want{Ghost, false, tgDoor, "tim"}},
		{"O6 listen", "O6", 0, func(g *Game) {}, want{Human, false, tgGuest, "teo"}},
		{"O6 reveal", "O6", 0, func(g *Game) {
			g.saw("teo")
			p := g.arriveGhost(1)
			p.Guests, p.Nights = p.Guests[:1], 3
			g.settleIn(p)
			g.rollSet("O6.room", 2)
		}, want{Ghost, false, tgGuest, "porter"}},
		{"O6 call Father K", "O6", 0, func(g *Game) { g.saw("teo"); g.rollSet("O6.room", 2) }, want{Human, false, tgDoor, "porter"}},
		{"O7 find", "O7", 0, func(g *Game) {}, want{Ghost, false, tgGuest, "osvaldo"}},
		{"O7 move", "O7", 0, func(g *Game) { g.saw("osvaldo") }, want{Human, false, tgGuest, "magda"}},
		{"O8 buy", "O8", 0, func(g *Game) {}, want{Human, true, tgTerminal, ""}},
		{"O8 parcel", "O8", 0, func(g *Game) { g.rollSet("O8.parcel", 1) }, want{Ghost, false, tgDoor, "celestine"}},
		{"O8 give", "O8", 0, func(g *Game) { g.verbs.held = "O8" }, want{Ghost, false, tgGuest, "celestine"}},
		{"O9 ask", "O9", 0, func(g *Game) {}, want{Ghost, false, tgGuest, "tim"}},
		{"O9 find", "O9", 0, func(g *Game) { g.saw("O9.read") }, want{Human, false, tgNone, ""}},
		{"O9 give", "O9", 0, func(g *Game) { g.verbs.held = "O9" }, want{Ghost, false, tgGuest, "tim"}},
		{"O10 find", "O10", 0, func(g *Game) {}, want{Ghost, false, tgGuest, "celestine"}},
		{"O10 move", "O10", 0, func(g *Game) { g.saw("celestine") }, want{Human, false, tgGuest, "magenta"}},
		{"O11 find", "O11", 0, func(g *Game) {}, want{Ghost, false, tgGuest, "ally"}},
		{"O11 give", "O11", 0, func(g *Game) { g.verbs.held = "O11" }, want{Human, false, tgGuest, "magenta"}},
		{"O12 odd", "O12", 0, func(g *Game) {}, want{Human, false, tgBook, ""}},
		{"O12 name", "O12", 0, func(g *Game) { g.saw("O12.odd") }, want{Ghost, false, tgBook, ""}},
		{"O13 buy", "O13", 0, func(g *Game) {}, want{Human, true, tgTerminal, ""}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			beat := slices.IndexFunc(beats[:], func(b beat) bool {
				return slices.ContainsFunc(b.Objs, func(o objective) bool { return o.ID == c.obj })
			}) + 1
			g, _ := verbRun(t, 3, beat)
			g.level = c.level
			c.set(g)
			s, ok := g.stepOf(c.obj)
			if !ok || s.Dim != c.want.dim || s.Any != c.want.any || s.Tgt.Kind != c.want.kind || s.Tgt.Key != c.want.key {
				t.Errorf("step = %+v (%v), want %+v", s, ok, c.want)
			}
		})
	}
}

// Every row fits the checklist (32 characters) with the longest fill-ins, and every objective has steps.
func TestStepTextsFit(t *testing.T) {
	g, _ := verbRun(t, 3, 1)
	for _, b := range beats {
		for _, o := range b.Objs {
			if len(objSteps[o.ID]) == 0 {
				t.Errorf("%s has no steps", o.ID)
			}
		}
	}
	for id, ss := range objSteps {
		for _, s := range ss {
			if txt := g.stepText(id, s); len(txt) > 32 {
				t.Errorf("%s %q is %d characters", id, txt, len(txt))
			}
		}
	}
}

// Before the first Growth and between beats the checklist points at the Growth; a done objective is ticked.
func TestGuideRowsGrow(t *testing.T) {
	g, _ := verbRun(t, 2, 1)
	g.quest = questRun{}
	if _, rows := g.guideRows(); len(rows) != 1 || rows[0].ID != "O0" {
		t.Errorf("before the first Growth: %+v", rows)
	}
	setQuest(g, 1)
	g.level = 2
	g.quest.Done[0] = true
	if _, rows := g.guideRows(); len(rows) != 2 || !rows[0].Done || rows[1].ID != "O0" {
		t.Errorf("between beats: %+v", rows)
	}
}

// The checklist shows for questHold after a change (beat, objective, Dimension), then is gone; the hotel view always shows it.
func TestQuestHudShowRule(t *testing.T) {
	g, _ := verbRun(t, 2, 1)
	g.state = StatePlay
	setQuest(g, 1)
	age := func(n int) float32 {
		for ; n > 0; n-- {
			g.updateHUD()
		}
		return g.questHud.alpha(g)
	}
	if a := age(1); a != 1 {
		t.Fatalf("after a beat starts: alpha %v", a)
	}
	if a := age(questHold + questFade); a != 0 {
		t.Fatalf("after the window: alpha %v", a)
	}
	g.state = StateView
	if a := g.questHud.alpha(g); a != 1 {
		t.Fatalf("in the hotel view: alpha %v", a)
	}
	g.state = StatePlay
	g.quest.Done[0] = true
	if a := age(1); a != 1 {
		t.Fatalf("after an objective is done: alpha %v", a)
	}
	age(questHold + questFade)
	g.ghost = !g.ghost
	if a := age(1); a != 1 {
		t.Fatalf("after the Dimension shifts: alpha %v", a)
	}
}

// lum: a colour's relative luminance (WCAG).
func lum(r, g, b uint8) float64 {
	f := func(v uint8) float64 {
		c := float64(v) / 255
		if c <= 0.03928 {
			return c / 12.92
		}
		return math.Pow((c+0.055)/1.055, 2.4)
	}
	return 0.2126*f(r) + 0.7152*f(g) + 0.0722*f(b)
}

// The quest ink reads on the page (4.5:1, even in the gutter's shade) and no regular pen is it.
func TestQuestInkContrast(t *testing.T) {
	paper := lum(161, 158, 148)
	q := lum(questInk.R, questInk.G, questInk.B)
	if c := (paper + .05) / (q + .05); c < 4.5 {
		t.Errorf("quest ink contrast %.2f", c)
	}
	if slices.Contains(bookInks[:], questInk) {
		t.Error("a regular pen is the quest ink")
	}
	if !(bookEntry{Out: outQuest}).quest() || !(bookEntry{Line: nameEntry, Out: outCozy}).quest() || (bookEntry{Out: outCozy}).quest() {
		t.Error("quest() wrong")
	}
}

// O12's entry sits on a later spread: the way there is D, opening the book and reading another page ticks nothing,
// and turning to its spread does.
func TestBookWayToQuestPage(t *testing.T) {
	g, _ := verbRun(t, 2, 6)
	for i := range 30 {
		g.guestBook = append(g.guestBook, bookEntry{"Guest", notebookCozy[i%len(notebookCozy)], 1, outCozy})
	}
	g.rollSet("O12.at", 25)
	g.openBook()
	if g.sawn("O12.odd") || g.bookWay() != 1 {
		t.Fatalf("on open: saw %v, way %d", g.sawn("O12.odd"), g.bookWay())
	}
	for i := 0; i < 20 && g.bookWay() == 1; i++ {
		g.in.(*script).hold()
		g.in.(*script).hold(ebiten.KeyD)
		g.updateBook()
		if g.bookWay() == 1 && g.sawn("O12.odd") {
			t.Fatal("ticked on another page")
		}
	}
	if g.bookWay() != 0 || !g.sawn("O12.odd") {
		t.Errorf("way %d, saw %v", g.bookWay(), g.sawn("O12.odd"))
	}
}

// A step that needs Father M's call outlines the Terminal and his STAFF row until it is bought.
func TestNeedsRow(t *testing.T) {
	g, _ := verbRun(t, 3, 4)
	g.saw("teo")
	g.rollSet("O6.room", 2)
	g.locked[unlockM] = true
	if ids := g.needRows(); !slices.Contains(ids, unlockM) {
		t.Fatalf("rows %v", ids)
	}
	if !g.onSale(row(t, unlockM)) {
		t.Error("the row the step needs is not on sale")
	}
	g.unlock(unlockM)
	if g.stepNeeds(unlockM) {
		t.Error("still needed")
	}
	g.locked[unlockM] = true
	g.rollSet("O6.room", 0)
	if g.onSale(row(t, unlockM)) {
		t.Error("on sale with no step needing it and its gate unearned")
	}
	g.unlock(unlockM)
	if ids := g.needRows(); slices.Contains(ids, unlockM) {
		t.Fatalf("still needed: %v", ids)
	}
}

// A quest guest in a fogged Room keeps its outline, drawn hollow over the Fog; out of it, the outline is the old one.
func TestOutlineUnderFog(t *testing.T) {
	g, _ := verbRun(t, 2, 1)
	p := named(t, g, 0, "magenta")
	guest := p.Guests[0]
	sh := g.hotel.Rooms[0].Shell()
	guest.X, guest.Y = float64(sh.Min.X+40), float64(sh.Min.Y+40)
	m := mark{guest: guest, party: p}
	g.fog = make([]float64, len(g.hotel.Rooms))
	if g.underFog(m) || len(g.guideActors([]mark{m}, 0, 0)) != 1 {
		t.Fatal("clear room: not hollow")
	}
	g.fog[0] = 1
	if !g.underFog(m) || len(g.guideActors([]mark{m}, 0, 0)) != 0 {
		t.Fatal("fogged room: drawn under the Fog")
	}
}

// 10: a step at the Terminal (the grow step, O8's buy) never outlines the Terminal; its "!" shows instead.
func TestTerminalStepNoOutline(t *testing.T) {
	o8 := slices.IndexFunc(beats[:], func(b beat) bool {
		return slices.ContainsFunc(b.Objs, func(o objective) bool { return o.ID == "O8" })
	}) + 1
	for _, beat := range []int{0, o8} { // GROW THE HOTEL; O8's buy
		g, _ := verbRun(t, 3, max(beat, 1))
		if beat == 0 {
			g.quest = questRun{} // before the first beat: the grow step
		}
		if !g.terminalStep() {
			t.Fatalf("beat %d: no step at the Terminal", beat)
		}
		w, h := g.mapSize()
		for _, m := range g.guideMarks(image.Rect(0, 0, w, h)) {
			if m.box.Overlaps(g.hotel.Terminal) {
				t.Errorf("beat %d: the Terminal is outlined (%v)", beat, m.box)
			}
		}
	}
}

// The grow step outlines nothing in the Terminal (live test 3: the tab's "!" is enough).
func TestNeedsGrowRows(t *testing.T) {
	g, _ := verbRun(t, 2, 1)
	g.quest = questRun{}
	if ids := g.needRows(); slices.Contains(ids, GrowCol) || slices.Contains(ids, GrowFloor) {
		t.Errorf("grow step open: %v", ids)
	}
}
