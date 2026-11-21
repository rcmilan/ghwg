package game

import (
	"slices"

	"ghosthotel/entities"
)

// 08 W5-VERBS: the Card's quest side (TELL, TAKE, GIVE, a quest guest's Ask line, REVEAL's quest facts), the talk's
// target override and the guest book's quest entries.

// verbOpened: a Card was opened. Morwenna's, while O1 is open, gives the Concierge the word to carry across.
func (g *Game) verbOpened(p *Party) {
	g.verbSync()
	if g.questOpen("O1") && p.Realm == Human && p.Guests[0].Who.Key == "morwenna" {
		g.verbs.held = "O1"
	}
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

// verbOpts: the quest options of this guest's Card now, in order: TELL, TAKE, GIVE. Only what applies.
func (g *Game) verbOpts(p *Party, guest *entities.Guest) (os []cardOpt) {
	g.verbSync()
	k, h := guest.Who.Key, g.verbs.held
	if h == "O1" && g.questOpen("O1") && p.Realm == Ghost && p.Room == g.roomOfKey("morwenna") || h == "O2" && g.questOpen("O2") && k == "morwenna" {
		os = append(os, optTell)
	}
	if g.takeId(p, guest) != "" {
		os = append(os, optTake)
	}
	if h == "O9" && g.questOpen("O9") && k == "ned" || h == "O11" && g.questOpen("O11") && k == "morwenna" {
		os = append(os, optGive)
	}
	return os
}

// verbOptText: the row of a quest option.
func (g *Game) verbOptText(o cardOpt, p *Party, guest *entities.Guest) string {
	switch o {
	case optTell:
		return "TELL THE " + heldNames[g.verbs.held]
	case optGive:
		return "GIVE THE " + heldNames[g.verbs.held]
	case optTake:
		return "TAKE THE " + heldNames[g.takeId(p, guest)]
	}
	return macguffinCard.Reveal // optNoReveal
}

// cardVerb: SPACE on a quest option. The Card stays open and says the done line where the guest has one; O9's
// GIVE closes it for the cutscene.
func (g *Game) cardVerb(o cardOpt, p *Party, guest *entities.Guest, c *card) {
	switch o {
	case optTell: // O1: the ghost roommate hears the word; O2: Morwenna hears the entry
		id := g.verbs.held
		g.verbs.held = ""
		c.say = objOf(id).Done
		g.verbDone(evTell, p.Room, id)
	case optTake:
		id := g.takeId(p, guest)
		g.rollSet(id+".took", 1)
		g.verbs.held = id
		c.say = ""
	case optGive:
		id := g.verbs.held
		g.verbs.held = ""
		if id == "O9" {
			g.endTalk()
			g.verbDone(evGive, p.Room, id)
			g.playQuestScene("O9")
			return
		}
		c.say = objOf(id).Done
		g.verbDone(evGive, p.Room, id)
	}
}

// verbQuote: what the guest's Card says instead of its regular line: the line it was last given (c.say), else the
// Ask of the open objective that is this guest's.
func (g *Game) verbQuote(p *Party, guest *entities.Guest, c *card) string {
	if c.say != "" {
		return c.say
	}
	g.verbSync()
	switch id := g.takeId(p, guest); {
	case id == "O9":
		return keyGive
	case id == "O11":
		return objOf(id).Ask
	}
	switch k := guest.Who.Key; {
	case k == "morwenna" && p.Realm == Human:
		for _, id := range [...]string{"O1", "O2", "O3", "O7"} {
			if g.questOpen(id) {
				if id == "O1" && g.rollGet("O1.long") == 1 {
					return o1LongAsk
				}
				return objOf(id).Ask
			}
		}
	case k == "ned" && g.questOpen("O5"):
		return objOf("O5").Ask
	case k == "teo" && g.questOpen("O6"):
		return g.fill(objOf("O6").Ask, g.porterRoom())
	}
	return ""
}

// verbRead: what REVEAL reads of a quest guest: the quirk and the fact ("" for the card's own). O6's porter reads
// NIGHT PORTER; O9's Ned gives the key's whereabouts.
func (g *Game) verbRead(p *Party, guest *entities.Guest) (quirk, fact string) {
	if p.Realm == Ghost && g.questOpen("O6") && p == g.porterParty() && guest == g.verbs.porter {
		return porterQuirk, porterFact
	}
	if guest.Who.Key == "ned" && g.questOpen("O9") {
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
	case k == "morwenna" && g.questOpen("O10"):
		return g.roomOfKey("celestine")
	}
	return -1
}

// withTarget: the talk's target Rooms ts with the objective's first, even a bad pair; unless the Room can't take the
// Party at all (not owned, shut, changing, too small, or held by a human Party that can't swap).
func (g *Game) withTarget(p *Party, ts []int) []int {
	q := g.questTarget(p)
	if q < 0 || q == p.Room {
		return ts
	}
	ts = slices.DeleteFunc(ts, func(i int) bool { return i == q })
	if !g.isOwned(q) || g.closed(q) || g.changing(q) || len(g.hotel.Rooms[q].Stays) < len(p.Guests) {
		return ts
	}
	if h := g.humanIn(q); h != nil && (!movable(h) || len(g.hotel.Rooms[p.Room].Stays) < len(h.Guests)) {
		return ts
	}
	return slices.Insert(ts, 0, q)
}

// celestineName: Madame's name in the book, short enough to leave the entry's tag room.
const celestineName = "Mme Arbogast"

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
