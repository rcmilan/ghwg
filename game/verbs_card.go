package game

import (
	"math"
	"slices"

	"ghosthotel/entities"
)

// 08 W5-VERBS: quest TALK (TELL, TAKE, GIVE, a quest guest's Ask line), the Card's REVEAL quest facts, the talk's
// target override and the guest book's quest entries.

// verbOpened: a Card was opened: its guests have been seen.
func (g *Game) verbOpened(p *Party) {
	g.verbSync()
	for _, guest := range p.Guests {
		g.saw(guest.Who.Key)
	}
}

const (
	talkReach = 24 // px: how close the Concierge must be to a quest character to talk to it
	talkHold  = 90 // frames a quest TALK keeps the character standing
)

// isQuestChar: the guest is one SPACE talks to as a quest character: a named one of the questline, or one with a
// word or thing to take or give now (the ghost roommate, the oldest sleeper).
func (g *Game) isQuestChar(p *Party, guest *entities.Guest) bool {
	return slices.Contains(questCast, guest.Who.Key) || len(g.verbOpts(p, guest)) > 0
}

// questTalk: the quest character of the Concierge's Dimension nearest to him within reach, in his Room or both out
// of any; walking or settled. nil if none.
func (g *Game) questTalk() (*Party, *entities.Guest) {
	var bp *Party
	var bg *entities.Guest
	best, room := float64(talkReach), g.roomAt(g.player.X, g.player.Y)
	for _, p := range g.parties {
		if p.Realm != g.realm() || p.abed || g.tick < p.hold { // one already stopped for a talk is not talked to again until it moves
			continue
		}
		for _, guest := range p.Guests {
			if d := math.Hypot(guest.X-g.player.X, guest.Y-g.player.Y); d <= best && p.shown(guest) && g.roomAt(guest.X, guest.Y) == room && g.isQuestChar(p, guest) {
				bp, bg, best = p, guest, d
			}
		}
	}
	return bp, bg
}

// questSpeak: SPACE at a quest character. It stops and faces the Concierge, the camera closes in on it (the "talk"
// cutscene, questscene.go) and it says its quest line; then the word or thing the Concierge carries that it needs
// goes over, what it gives is taken (questHand) and its done line follows. Nothing quest-related to say: one of its
// own lines.
func (g *Game) questSpeak(p *Party, guest *entities.Guest) {
	g.verbSync()
	p.hold = g.tick + talkHold
	guest.Anim.Facing = entities.Toward(g.player.X-guest.X, g.player.Y-guest.Y)
	g.verbOpened(p)
	if g.verbs.held == "O9" && slices.Contains(g.verbOpts(p, guest), optGive) { // the key to Tim: its cutscene is the talk, one close-in, no ask again (live test 5)
		g.cardVerb(optGive, p, guest)
		return
	}
	g.playQuestScene("talk")
	s := &g.scene
	s.tp, s.tg, s.line, s.lineAt = p, guest, g.verbQuote(p, guest), sceneIn
}

// questHand: the talk's hand-over, after the quest line: Magenta's word to carry, or the quest verb that applies.
// It returns the done line, if any.
func (g *Game) questHand(p *Party, guest *entities.Guest) string {
	if g.questOpen("O1") && p.Realm == Human && guest.Who.Key == "magenta" { // her word to carry across
		g.verbs.held = "O1"
	}
	if os := g.verbOpts(p, guest); len(os) > 0 {
		return g.cardVerb(os[0], p, guest)
	}
	return ""
}

// ownLine: one of the guest's own lines, in turn ("" none).
func ownLine(p *Party, guest *entities.Guest) string {
	n := len(guest.Who.Lines)
	if n == 0 {
		return ""
	}
	p.said++
	return guest.Who.Lines[(p.said-1)%n]
}

// takeId: the objective whose thing this guest has to hand over now ("" none): the oldest sleeper's key (O9), the
// rolled ghost ally's HAFT (O11). Only with the hands free: the Concierge carries one.
func (g *Game) takeId(p *Party, guest *entities.Guest) string {
	switch {
	case g.verbs.held != "":
	case g.questOpen("O9") && g.rollGet("O9.took") == 0 && p.Realm == Human && p == g.verbs.oldest && guest.Who.Key == "":
		return "O9"
	case g.questOpen("O11") && g.rollGet("O11.took") == 0 && p.Realm == Ghost && guest.Who.Key != "" && guest.Who.Key == g.ally():
		return "O11"
	}
	return ""
}

// verbOpts: the quest verbs that apply to this guest now, in order: TELL, TAKE, GIVE. Only what applies.
func (g *Game) verbOpts(p *Party, guest *entities.Guest) (os []cardOpt) {
	g.verbSync()
	k, h := guest.Who.Key, g.verbs.held
	if h == "O1" && g.questOpen("O1") && p.Realm == Ghost && p.Room == g.roomOfKey("magenta") || h == "O2" && g.questOpen("O2") && k == "magenta" {
		os = append(os, optTell)
	}
	if g.takeId(p, guest) != "" {
		os = append(os, optTake)
	}
	if h == "O9" && g.questOpen("O9") && k == "tim" || h == "O11" && g.questOpen("O11") && k == "magenta" {
		os = append(os, optGive)
	}
	return os
}

// cardVerb: the quest verb o done, the guest's done line returned where it has one; O9's GIVE plays its cutscene.
func (g *Game) cardVerb(o cardOpt, p *Party, guest *entities.Guest) string {
	switch o {
	case optTell: // O1: the ghost roommate hears the word; O2: Magenta hears the entry
		id := g.verbs.held
		g.verbs.held = ""
		g.verbDone(evTell, p.Room, id)
		return objOf(id).Done
	case optTake:
		id := g.takeId(p, guest)
		g.rollSet(id+".took", 1)
		g.verbs.held = id
	case optGive:
		id := g.verbs.held
		g.verbs.held = ""
		g.verbDone(evGive, p.Room, id)
		if id == "O9" {
			g.playQuestScene("O9")
			return ""
		}
		return objOf(id).Done
	}
	return ""
}

// verbQuote: what the guest says of its quest, or on its Card instead of its regular line: the Ask of the open
// objective that is this guest's.
func (g *Game) verbQuote(p *Party, guest *entities.Guest) string {
	g.verbSync()
	switch id := g.takeId(p, guest); {
	case id == "O9":
		return keyGive
	case id == "O11":
		return objOf(id).Ask
	}
	switch k := guest.Who.Key; {
	case k == "magenta" && p.Realm == Human:
		for _, id := range [...]string{"O1", "O2", "O3", "O7", "O11"} {
			if g.questOpen(id) {
				if id == "O1" && g.rollGet("O1.long") == 1 {
					return o1LongAsk
				}
				if id == "O11" {
					return o11Magenta
				}
				return objOf(id).Ask
			}
		}
	case k == "tim" && g.questOpen("O5"):
		return objOf("O5").Ask
	case k == "tim" && g.questOpen("O9"): // the talk is the asking: the step ticks
		g.saw("O9.read")
		return objOf("O9").Ask
	case k == "teo" && g.questOpen("O6"):
		return g.fill(objOf("O6").Ask, g.porterRoom())
	}
	return ""
}

// verbRead: what REVEAL reads of a quest guest: the quirk and the fact ("" for the card's own). O6's porter reads
// NIGHT PORTER; O9's Tim gives the key's whereabouts.
func (g *Game) verbRead(p *Party, guest *entities.Guest) (quirk, fact string) {
	if p.Realm == Ghost && g.questOpen("O6") && p == g.porterParty() && guest == g.verbs.porter {
		return porterQuirk, porterFact
	}
	if guest.Who.Key == "tim" && g.questOpen("O9") {
		g.saw("O9.read")
		return "", objOf("O9").Ask
	}
	return "", ""
}

// questTarget: the Room the open objective wants this human Party moved to, -1 if none (O3, O7, O10).
func (g *Game) questTarget(p *Party) int {
	if p.Realm != Human || len(p.Guests) == 0 {
		return -1
	}
	switch k := p.Guests[0].Who.Key; {
	case k == "magda" && g.questOpen("O3"):
		return g.roomOfKey("celestine")
	case k == "magda" && g.questOpen("O7"):
		return g.roomOfKey("osvaldo")
	case k == "magenta" && g.questOpen("O10"):
		return g.roomOfKey("celestine")
	}
	return -1
}

// withTarget: the talk's target Rooms ts with the objective's first, even a bad pair (even a closed Room); unless the Room can't take the
// Party at all (not owned, changing, too small, or held by a human Party that can't swap).
func (g *Game) withTarget(p *Party, ts []int) []int {
	q := g.questTarget(p)
	if q < 0 || q == p.Room {
		return ts
	}
	ts = slices.DeleteFunc(ts, func(i int) bool { return i == q })
	if !g.isOwned(q) || g.changing(q) || len(g.hotel.Rooms[q].Stays) < len(p.Guests) {
		return ts
	}
	if h := g.humanIn(q); h != nil && (!movable(h) || len(g.hotel.Rooms[p.Room].Stays) < len(h.Guests)) {
		return ts
	}
	return slices.Insert(ts, 0, q)
}

// celestineName: Madame's name in the book, short enough to leave the entry's tag room.
const celestineName = nameCelestineShort

// The guest book's quest entries, shown only in the Dimension they belong to. outQuest ones are ghost-only.

// questEntries: the open objectives' entries put into the page's list (newest first): Celestine's note (O2), in
// ghost, on top, and at Lv2+ its end on the oldest spread; O12's entry at its rolled place, odd capitals in real, the
// MacGuffin's name in ghost.
func (g *Game) questEntries(es []bookEntry) []bookEntry {
	if g.ghost && g.questOpen("O2") {
		if room := g.roomOfKey("celestine"); room >= 0 {
			es = slices.Insert(es, 0, bookEntry{celestineName, g.fill(celestineEntry, room), day(g.clock), outQuest})
			if g.level >= 2 {
				es = append(es, bookEntry{celestineName, celestineEntryHard, day(g.clock), outQuest})
			}
		}
	}
	if g.questOpen("O12") {
		at := min(max(g.rollGet("O12.at")-1, 0), len(es))
		e := bookEntry{"Anon.", nameEntry, day(g.clock), outCozy}
		if g.ghost {
			e = bookEntry{"Anon.", macguffinName, day(g.clock), outQuest}
		}
		es = slices.Insert(es, at, e)
	}
	return es
}

// bookRead: the page the Concierge has open in ghost shows quest entries: reading Celestine's gives him her word to
// carry (both parts at Lv2+, on their spreads); reading the name finishes O12.
func (g *Game) bookRead() {
	if !g.ghost {
		if es := g.shown(); g.questOpen("O12") {
			for i := range bookSpread(es, g.bookTop) {
				if es[g.bookTop+i].Line == nameEntry {
					g.saw("O12.odd")
				}
			}
		}
		return
	}
	g.verbSync()
	es := g.shown()
	for i := range bookSpread(es, g.bookTop) {
		e := es[g.bookTop+i]
		switch {
		case e.Out != outQuest:
		case e.Line == macguffinName:
			g.notice(objOf("O12").Done)
			g.verbDone(evBook, -1, "O12")
		case e.Line == celestineEntryHard:
			g.rollSet("O2.read", g.rollGet("O2.read")|2)
		case e.Name == celestineName:
			g.rollSet("O2.read", g.rollGet("O2.read")|1)
		}
	}
	if need := 1 + 2*b2i(g.level >= 2); g.questOpen("O2") && !slices.Contains(g.verbs.done, "O2") && g.rollGet("O2.read")&need == need {
		g.verbs.held = "O2"
	}
}

// reserved: Madame's Room while O3 or O10 still needs its human guest (Magda, Magenta) moved into it: no other
// human books or swaps into it, so once its occupant leaves it stays free for the quest guest.
func (g *Game) reserved(room int) bool {
	if room < 0 || room != g.roomOfKey("celestine") {
		return false
	}
	if g.crateHold() {
		return true
	}
	for _, c := range [...]struct{ id, key string }{{"O3", "magda"}, {"O10", "magenta"}} {
		if p := g.questParty(c.key); p != nil && g.questOpen(c.id) && p.Room != room {
			return true
		}
	}
	return false
}

// crateBought: a courier is carrying the crate, or it is down and waiting for the finale.
func (g *Game) crateBought() bool { return g.verbsComing(rowCrate) || g.crateIn() }

// crateHold: the crate is bought and the finale not done: nobody is booked into Madame's Room (reserved, and the
// ghost arrivals in ghostguest.go). The finale ends in a prestige, so nothing lifts it.
func (g *Game) crateHold() bool {
	return g.crateBought() || g.scene.id == "finale" || g.scene.id == "credits"
}

// clearMadame: the crate is bought: every Party in Madame's Room but the finale's cast leaves at once, by the
// ordinary check-out walk.
func (g *Game) clearMadame() {
	room := g.roomOfKey("celestine")
	for _, p := range g.parties {
		if p.Room != room || p.state == checkingOut || p.state == leaving || p.Paid ||
			slices.ContainsFunc(p.Guests, func(x *entities.Guest) bool { return slices.Contains(objCast["O13"], x.Who.Key) }) {
			continue
		}
		p.state = checkingOut
		g.sendParty(p, g.hotel.Exit, g.hotel.Exit)
	}
}

// questRoom: a Priest keeps out of it: reserved, or holding a Party a quest needs where it is.
func (g *Game) questRoom(room int) bool {
	return g.reserved(room) || slices.ContainsFunc(g.parties, func(p *Party) bool { return p.Room == room && g.questPinned(p) })
}

// questHeld: a quest character's Party, or one an open objective wants where it is: the housekeepers never move it.
func (g *Game) questHeld(p *Party) bool {
	return g.questPinned(p) || slices.ContainsFunc(p.Guests, func(guest *entities.Guest) bool { return slices.Contains(questCast, guest.Who.Key) })
}
