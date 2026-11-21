package game

import (
	"image"
	"slices"
	"strings"

	"ghosthotel/world"
)

// 08 W5: Magenta's circle, the questline's state (W5-SEAM). The data is circle.go. W5-QUEST fills the stubs below;
// every verb (verbs.go) reports what it did through questEvent, the one call the engine listens to.

// questRun: this Run's questline, in the dawn checkpoint (runState.Quest).
type questRun struct {
	Beat  int            // the beat under way, 1..beatCount; 0 before the first Growth
	Done  []bool         // per objective of Beat, in beats[Beat-1].Objs order
	Rolls map[string]int // this Run's rolled parts, by name ("room", "porter", ...): the engine's to name
	At    int            // the clock Beat started
	Said  bool           // its opening line was said (it waits for its speaker to be in the hotel)

	paid   Num    // purses paid so far (the sim's Ecto check reads it)
	naming bool   // questCheckIn is booking: no other named guest rolls first
	owe    string // a beat-end line its Speaker was away for: it comes back to say it (not saved)
	oweBy  string
}

// questRecord: the questline across every Run, saved with the Level and the Bonus (saveData.Quest).
type questRecord struct {
	Times     [beatCount]int  // times each beat was done, ever
	Items     [itemCount]bool // the items owned (given the first time ever a beat is done)
	MacGuffin bool            // the finale was played: the MacGuffin is summoned, once ever
}

// questEv is a quest-worthy thing a verb did.
type questEv int

const (
	evGrow    questEv = iota // a Growth landed (beginGrow): a beat starts
	evDawn                   // 08:00 (the checkpoint): overnight objectives check
	evTell                   // TELL on a Card: room, the guest's key
	evGive                   // GIVE on a Card (the Concierge gives): room, key
	evTake                   // TAKE on a Card (the Concierge takes): room, key
	evReveal                 // a REVEAL: room, key
	evMove                   // a talk MOVE landed: room is the new Room, key
	evServe                  // a ghost Request delivered: room, key
	evDeliver                // a quest row delivered (the wreath, the crate): room (-1: the Mirror), key the row's
	evLit                    // the spirit caught in the Lantern beam: room -1, key ""
	evDoor                   // SPACE on the spirit's handprint door in ghost: room
	evRitual                 // a Priest's Ritual landed: room, key "k" or "m"
	evBook                   // a quest entry read in the guest book: room -1, key the entry's
	evScene                  // a cutscene ended: room -1, key its id (information: it ticks nothing)
	evDone                   // an objective done outside play (dev starts, tests, the quester Strategy): key its ID
)

// questEvent: a verb did something quest-worthy (room -1 when none). The contract (W5-SEAM): VERBS plays each
// objective and, the moment one is finished, reports it with key = its ID ("O1".."O13"); QUEST ticks an open
// objective so reported, whatever ev but evScene. Other keys are information (evGrow, evDawn, ...).
func (g *Game) questEvent(ev questEv, room int, key string) {
	if ev == evGrow {
		g.questGrow()
		return
	}
	if ev == evScene || !g.questOpen(key) { // a scene id ("O4") names its objective, which only the verb finishes
		return
	}
	q := &g.quest
	for i, o := range beats[q.Beat-1].Objs {
		if o.ID == key {
			q.Done[i] = true
		}
	}
	if !slices.Contains(q.Done, false) {
		g.beatDone()
	}
}

// questStart: a Run starts (startRun): the questline back to before beat 1, rolled again.
func (g *Game) questStart() { g.quest = questRun{Rolls: map[string]int{}} }

// questGrow: a Growth landed: the next beat of this Run starts, if the Run's level plays one more.
func (g *Game) questGrow() {
	q := &g.quest
	if q.Rolls == nil {
		q.Rolls = map[string]int{}
	}
	if q.Beat >= beatsAt(g.level) {
		return
	}
	q.Beat++
	bt := beats[q.Beat-1]
	q.Done, q.At, q.Said = make([]bool, len(bt.Objs)), g.clock, bt.AtEnd
	g.questSay()
}

// questSay: the beat's opening line, said by its Speaker over its guest as soon as that guest is in the hotel.
func (g *Game) questSay() {
	q := &g.quest
	if q.owe != "" && g.sayLine(q.oweBy, q.owe) {
		q.owe = ""
	}
	if q.Beat < 1 || q.Said {
		return
	}
	bt := beats[q.Beat-1]
	if text := byTimes(bt.Lines, g.timesDone(q.Beat)); text == "" || g.sayLine(bt.Speaker, text) {
		q.Said = true
	}
}

// sayLine: the quest guest key says text over its head, a balloon at a time (they fit one short line); false if it
// is not in the hotel.
func (g *Game) sayLine(key, text string) bool {
	p, who := g.questHolder(key)
	if p == nil {
		return false
	}
	speak(p, who, text, g.tick)
	return true
}

// speak: member who of p says text over its head from tick at, a balloon at a time.
func speak(p *Party, who int, text string, at int) {
	for k, chunk := range wrapAt(text, sayWrap) {
		p.talk = append(p.talk, line{who, chunk, at + k*sayFrames})
	}
}

// sayWrap: characters a quest line's balloon holds (a balloon is 6 px a character at x3, the screen 320 px wide).
const sayWrap = 34

// beatDone: the beat under way is done: it counts, and the first time ever it pays its purse and gives its item.
func (g *Game) beatDone() {
	b := g.quest.Beat
	bt, n := beats[b-1], g.timesDone(b)
	g.questRec.Times[b-1]++
	g.notice("BEAT DONE: " + bt.Title)
	if n == 0 {
		if bt.Purse > 0 {
			g.earn(Ecto, bt.Purse, image.Pt(int(g.player.X), int(g.player.Y)-world.TileSize))
			g.quest.paid += bt.Purse
		}
		if bt.Item != noItem {
			g.questRec.Items[bt.Item] = true
			g.notice("GOT " + strings.ToUpper(itemNames[bt.Item][0]))
		}
	}
	if line := byTimes(bt.Lines, n); bt.AtEnd && !g.sayLine(bt.Speaker, line) {
		g.quest.owe, g.quest.oweBy = line, bt.Speaker
	}
}

// questStage: how far this Run's questline has got: the beats done this Run (0..beatCount).
func (g *Game) questStage() int {
	q := g.quest
	if q.Beat < 1 {
		return 0
	}
	if slices.Contains(q.Done, false) {
		return q.Beat - 1
	}
	return q.Beat
}

// objCast: the quest guests each objective needs in the hotel (circle.md); O13's gather for the finale.
var objCast = map[string][]string{
	"O1": {"magenta"}, "O2": {"magenta", "celestine"}, "O3": {"magda", "celestine"}, "O5": {"tim"}, "O6": {"teo"},
	"O7": {"magda", "osvaldo"}, "O8": {"celestine"}, "O9": {"tim"}, "O10": {"magenta", "celestine"},
	"O11": {"magenta", "celestine", "tim"}, "O13": {"magenta", "celestine", "tim"},
}

// questNeeds: the guest key is needed now: by an open objective of the beat under way (circle.md: they stay until
// their objective is done, then leave), as its Speaker until its opening line is said, or for a line it owes.
func (g *Game) questNeeds(key string) bool {
	q := g.quest
	if q.Beat < 1 {
		return false
	}
	bt := beats[q.Beat-1]
	if q.owe != "" && key == q.oweBy || key == bt.Speaker && !q.Said {
		return true
	}
	for i, o := range bt.Objs {
		if i < len(q.Done) && !q.Done[i] && slices.Contains(objCast[o.ID], key) {
			return true
		}
	}
	return false
}

// questHolder: the Party holding the quest guest key, and the guest's index in it; nil if it is not in the hotel.
func (g *Game) questHolder(key string) (*Party, int) {
	for _, p := range g.parties {
		for i, guest := range p.Guests {
			if guest.Who.Key == key {
				return p, i
			}
		}
	}
	return nil, 0
}

// questPinned: the Party is a quest guest whose objective is not done: no checkout, flee, passing on or Father M.
func (g *Game) questPinned(p *Party) bool {
	for _, guest := range p.Guests {
		if g.questNeeds(guest.Who.Key) {
			return true
		}
	}
	return g.verbPinned(p)
}

// questGate: why the next Growth must wait ("SHIFT'S NOT OVER"), "" if it needn't.
func (g *Game) questGate() string {
	if q := g.quest; q.Beat >= 1 && q.Beat < beatsAt(g.level) && slices.Contains(q.Done, false) {
		return gateRefuse
	}
	return ""
}

// questArrivals runs each game minute, before the walk-ins: a quest guest of a beat so far that is still needed and
// not in the hotel checks in, one a minute (a Run resumed from a save has none in: they come back).
func (g *Game) questArrivals() {
	in := false
	for b := range g.quest.Beat {
		for _, key := range beats[b].Guests {
			if p, _ := g.questHolder(key); p == nil && (!in || StoryMode) && g.questNeeds(key) { // story: all at once
				in = g.questCheckIn(key) || in
			}
		}
	}
	g.questSay()
}

// questCheckIn books the named guest key into the next Vacant owned open Room that fits. False if none is free now.
func (g *Game) questCheckIn(key string) bool {
	g.quest.naming = true // arrive and arriveGhost roll no other named guest first
	defer func() { g.quest.naming = false }()
	if i := slices.IndexFunc(namedHumans, func(h namedHuman) bool { return h.Key == key }); i >= 0 {
		room := g.freeRoom(1)
		if room < 0 {
			g.storyOpen()
			room = g.freeRoom(1)
		}
		if room < 0 || !g.spawnClear() {
			return false
		}
		g.nameHuman(g.arrive(room), namedHumans[i])
		return true
	}
	i := slices.IndexFunc(namedGhosts, func(n namedGhost) bool { return n.Key == key })
	if i < 0 || !g.unlocked(Mirror) || !g.spawnClearIn(Ghost) {
		return false
	}
	room := -1
	for try := 0; try < 2 && room < 0; try++ {
		for k := range g.hotel.Rooms {
			if g.isOwned(k) && !g.closed(k) && !g.changing(k) && !g.stays(k) && g.vacant(k, Ghost) {
				room = k
				break
			}
		}
		if room < 0 && try == 0 {
			g.storyOpen()
		}
	}
	if room < 0 {
		return false
	}
	p := g.arriveGhost(room)
	g.nameGhost(p.Guests[0], namedGhosts[i])
	g.visited(p, key)
	return true
}

// itemOn: the item is owned (for good, across prestiges).
func (g *Game) itemOn(id itemID) bool { return id >= 0 && id < itemCount && g.questRec.Items[id] }

// questOpen: objective id ("O1".."O13") is in the beat under way and not done yet. VERBS plays an objective only
// while it is open, and reports it done with questEvent(ev, room, id).
func (g *Game) questOpen(id string) bool {
	if g.quest.Beat < 1 {
		return false
	}
	for i, o := range beats[g.quest.Beat-1].Objs {
		if o.ID == id {
			return i < len(g.quest.Done) && !g.quest.Done[i]
		}
	}
	return false
}

// timesDone: how many times beat b (1..beatCount) has been done, ever.
func (g *Game) timesDone(b int) int { return g.questRec.Times[b-1] }

// byTimes: the line for the n-th time (0: the 1st), the last non-empty one before it if that one is empty.
func byTimes(lines [3]string, n int) string {
	for i := min(n, 2); i >= 0; i-- {
		if lines[i] != "" {
			return lines[i]
		}
	}
	return ""
}

// DevQuest is main's -quest: the Run is at beat n (its level raised to play it), the hotel grown n times, the earlier
// beats done and every quest guest settled in. Dev only: the earlier beats pay their purses.
func (g *Game) DevQuest(n int) {
	if n < 1 {
		return
	}
	for g.level < 2 && beatsAt(g.level) < n {
		g.level++
	}
	g.unlock(Mirror)
	g.roomOwned = nil
	for b := 1; b <= n; b++ {
		if c, _ := g.hotel.Grid(); b%2 == 1 && c < maxGrid(g.level) {
			g.DevGrow("col")
		} else {
			g.DevGrow("floor")
		}
		for range 4 { // one guest a minute: each is settled in before the next
			g.questArrivals()
			for _, p := range g.parties {
				if p.state != staying {
					g.settleIn(p)
				}
			}
		}
		if b < n {
			for _, o := range beats[b-1].Objs {
				g.questEvent(evDone, -1, o.ID)
			}
		}
	}
	g.growing, g.state, g.viewT, g.note = nil, StatePlay, 0, notice{}
}
