package game

import (
	"slices"
	"testing"
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
		}, want{Human, false, tgDoor, "celestine"}},
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
		{"O6 call Father K", "O6", 0, func(g *Game) { g.saw("teo"); g.rollSet("O6.room", 2) }, want{Human, false, tgNone, ""}},
		{"O7 find", "O7", 0, func(g *Game) {}, want{Ghost, false, tgGuest, "osvaldo"}},
		{"O7 move", "O7", 0, func(g *Game) { g.saw("osvaldo") }, want{Human, false, tgGuest, "magda"}},
		{"O8 buy", "O8", 0, func(g *Game) {}, want{Human, true, tgTerminal, ""}},
		{"O8 parcel", "O8", 0, func(g *Game) { g.rollSet("O8.parcel", 1) }, want{Ghost, false, tgDoor, "celestine"}},
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
