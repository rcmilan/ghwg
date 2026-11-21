package game

import (
	"hash/fnv"
	"image"
	"slices"
	"strings"

	"ghosthotel/entities"
	"ghosthotel/world"

	"github.com/hajimehoshi/ebiten/v2"
)

// 08 W5-VERBS: the objective verbs (TELL, TAKE and GIVE on the Card, the ghost Request, the talk's target
// override, the quest rows, the quest guest book entries, the spirit with no ghost, REVEAL's quest facts, Father
// K's retirement party). Each reports its objective's end with verbDone -> questEvent. The Card side is
// verbs_card.go, the spirit spirit.go.

// verbState: the verbs' Run state, thrown away whenever the beat changes (verbSync). Per-Run rolled parts live in
// g.quest.Rolls ("O<n>.<part>"), which the checkpoint saves; this is only what the Concierge carries and who is who.
type verbState struct {
	beat     int             // the beat this state belongs to
	held     string          // the quest word or thing the Concierge carries: "O1", "O2" (words), "O9", "O11" (things); "" none
	done     []string        // objectives reported this beat
	oldest   *Party          // O9: the oldest human guest, who has the key
	porter   *entities.Guest // O6: the rolled ghost
	stranger *Party          // O3 Lv2+: the human in Madame's Room
	spirit   spiritRun       // O4 (spirit.go)
	litAt    image.Point     // O4: where the spirit was lit, world px, for the cutscene
	seen     map[string]bool // what the Concierge has seen this beat, for the checklist's next step (guide.go): a guest's Card opened by key, "O5.ask", "O9.read", "O12.odd"
}

// The word or thing a held id stands for, as the label over the Concierge's head says it.
var heldNames = map[string]string{"O1": "WORD", "O2": "WORD", "O9": "KEY", "O11": "HAFT"}

// verbSync drops the state of another beat or Run (a Run's start passes through beat 0).
func (g *Game) verbSync() {
	if g.verbs.beat != g.quest.Beat {
		g.verbs = verbState{beat: g.quest.Beat}
	}
}

// verbDone reports objective id finished, once.
func (g *Game) verbDone(ev questEv, room int, id string) {
	if slices.Contains(g.verbs.done, id) {
		return
	}
	g.verbs.done = append(g.verbs.done, id)
	g.questEvent(ev, room, id)
}

// The Run's rolls, "O<n>.<part>": 0 is not rolled, else the value + 1.
func (g *Game) rollGet(k string) int { return g.quest.Rolls[k] }
func (g *Game) rollSet(k string, v int) {
	if g.quest.Rolls == nil {
		g.quest.Rolls = map[string]int{}
	}
	g.quest.Rolls[k] = v
}

// roll: part k of the Run, 0..n-1, rolled once. From a hash of the clock, never an rng: nothing else's stream moves.
func (g *Game) roll(k string, n int) int {
	if v := g.rollGet(k); v > 0 && v <= n {
		return v - 1
	}
	h := fnv.New32a()
	h.Write([]byte(k))
	h.Write([]byte{byte(g.clock), byte(g.clock >> 8), byte(g.clock >> 16), byte(g.quest.Beat)})
	v := int(h.Sum32() % uint32(n))
	g.rollSet(k, v+1)
	return v
}

// objOf: the objective with this ID.
func objOf(id string) objective {
	for _, b := range beats {
		for _, o := range b.Objs {
			if o.ID == id {
				return o
			}
		}
	}
	return objective{}
}

// questParty: the Party of the named guest.
func (g *Game) questParty(key string) *Party {
	for _, p := range g.parties {
		for _, guest := range p.Guests {
			if guest.Who.Key == key {
				return p
			}
		}
	}
	return nil
}

// roomOfKey: the Room of the named guest, -1 if it is not in the hotel.
func (g *Game) roomOfKey(key string) int {
	if p := g.questParty(key); p != nil {
		return p.Room
	}
	return -1
}

// fill: s with {n} said as the Room's number.
func (g *Game) fill(s string, room int) string {
	if room < 0 || room >= len(g.hotel.Rooms) {
		return s
	}
	return strings.ReplaceAll(s, "{n}", g.hotel.Rooms[room].Number)
}

// together: the human a and the ghost b (named guests) are settled in one Room, a's Party arrived.
func (g *Game) together(a, b string) bool {
	pa, pb := g.questParty(a), g.questParty(b)
	return pa != nil && pb != nil && pa.Realm == Human && pb.Realm == Ghost && pa.Room == pb.Room && pa.state == staying && pb.state == staying
}

// sayDone: the guest's Party says an objective's done line: over its head if it is in the Dimension the Concierge is
// in, else on the HUD.
func (g *Game) sayDone(p *Party, text string) {
	if p != nil && p.Realm == g.realm() {
		p.talk = append(p.talk, line{0, text, g.tick})
		return
	}
	g.notice(text)
}

// questRowOn: id is a quest row of the Terminal (quest true); on: it is on sale now.
func (g *Game) questRowOn(id Unlock) (on, quest bool) {
	switch id {
	case rowWreath:
		return g.questOpen("O8") && g.roomOfKey("celestine") >= 0 && g.rollGet("O8.parcel") == 0 && !g.verbsComing(id), true
	case rowCrate:
		return g.questOpen("O13") && !g.verbsComing(id), true
	case rowStorm:
		return g.stormJarOn(), true
	}
	return false, false
}

// isQuestRow: the Terminal row is one of VERBS's orders (bought once, never an unlock).
func isQuestRow(id Unlock) bool { return id == rowWreath || id == rowCrate }

// verbsComing: a courier is carrying the quest row.
func (g *Game) verbsComing(id Unlock) bool {
	return slices.ContainsFunc(g.couriers, func(c *courier) bool {
		return slices.ContainsFunc(c.drops, func(d drop) bool {
			return slices.ContainsFunc(d.lines, func(l cartLine) bool { return shop[l.item].ID == id })
		})
	})
}

// shipQuest: the courier's stop for a bought quest row: the wreath at Madame's door, the crate at the Entrance Mirror.
func (g *Game) shipQuest(l cartLine) drop {
	if shop[l.item].ID == rowWreath {
		l.room = max(0, g.roomOfKey("celestine"))
		return drop{at: g.doorDrop(l.room), lines: []cartLine{l}}
	}
	return drop{at: g.hotel.Mirror, lines: []cartLine{l}}
}

// receiveQuest: the courier hands a quest row over: the wreath is left at the door (a parcel in real, the wreath in
// ghost); the crate reaches the Mirror, and the first time ever the finale plays.
func (g *Game) receiveQuest(l cartLine) {
	if shop[l.item].ID == rowWreath {
		g.rollSet("O8.parcel", l.room+1)
		return
	}
	first := !g.questRec.MacGuffin
	g.verbDone(evDeliver, -1, "O13")
	if first {
		g.playQuestScene("finale")
	}
}

// verbPinned: the Party is held by an objective VERBS plays (the porter, the oldest guest, ...): questPinned ORs it.
func (g *Game) verbPinned(p *Party) bool {
	switch {
	case g.questOpen("O6") && g.rollGet("O6.done") == 0 && p == g.porterParty():
		return true
	case g.questOpen("O9") && g.rollGet("O9.took") == 0 && p == g.verbs.oldest:
		return true
	case g.questOpen("O3") && p == g.verbs.stranger && p.Room == g.roomOfKey("celestine"):
		return true
	}
	return false
}

// porterParty: the Party of O6's rolled ghost, nil before it is rolled.
func (g *Game) porterParty() *Party {
	r := g.rollGet("O6.room") - 1
	if r < 0 {
		return nil
	}
	for _, p := range g.parties {
		if p.Realm == Ghost && p.Room == r && (g.verbs.porter == nil && p.Guests[0].Who.Key == "" || slices.Contains(p.Guests, g.verbs.porter)) {
			g.verbs.porter = p.Guests[0]
			return p
		}
	}
	return nil
}

// oldestParty: the human Party of the oldest generated guest in the hotel (named guests are the cast, not the sleepers).
func (g *Game) oldestParty() (best *Party) {
	age := 0
	for _, p := range g.parties {
		for _, guest := range p.Guests {
			if p.Realm == Human && guest.Who.Key == "" && guest.Who.Age > age {
				best, age = p, guest.Who.Age
			}
		}
	}
	return best
}

// questCast: the named guests the questline uses (cast.go).
var questCast = []string{"magenta", "magda", "teo", "celestine", "tim", "osvaldo"}

// allies: O11's ghost allies; the rolled one has the HAFT.
var allies = [...]string{"celestine", "tim", "osvaldo"}

func (g *Game) ally() string {
	if v := g.rollGet("O11.ally"); v > 0 {
		return allies[v-1]
	}
	return ""
}

// verbActors: the verbs' actors for the y-sorted pass: the spirit, the parcel and the handprint.
func (g *Game) verbActors() (as []actor) {
	if s := g.spiritActor(); s != nil {
		as = append(as, *s)
	}
	if r := g.rollGet("O8.parcel") - 1; r >= 0 && g.questOpen("O8") {
		c := g.doorDrop(r)
		as = append(as, actor{x: float64(c.X + world.TileSize), y: float64(c.Y), draw: func(s *ebiten.Image, x, y int) { g.drawParcel(s, x, y) }})
	}
	if r := g.handprintRoom(); r >= 0 && g.ghost {
		c := g.doorDrop(r)
		as = append(as, actor{x: float64(c.X), y: float64(c.Y), draw: func(s *ebiten.Image, x, y int) { g.drawHandprint(s, x, y-3*world.TileSize/2*zoom) }})
	}
	return as
}

// updateVerbs runs each step of world time.
func (g *Game) updateVerbs() {
	g.verbSync()
	g.updateSpirit()
	if p := g.questParty("tim"); g.ghost && g.questOpen("O5") && p != nil && p.asking() {
		g.saw("O5.ask")
	}
	if g.verbs.held != "" && !g.questOpen(g.verbs.held) {
		g.verbs.held = ""
	}
	if g.ticked && g.clock%minuteFrames == 0 {
		g.verbMinute()
	}
}

// verbMinute: each game minute, the parts that wait on the world: the rolls, who has what, the asks, the meetings
// and the overnight ones.
func (g *Game) verbMinute() {
	g.rollOpen()
	if g.questOpen("O5") {
		g.askTim()
	}
	if g.questOpen("O7") && g.together("magda", "osvaldo") {
		room := g.roomOfKey("osvaldo")
		g.sayDone(g.questParty("magda"), g.fill(objOf("O7").Done, room))
		g.verbDone(evMove, room, "O7")
	}
	switch g.clock % dayFrames {
	case 22 * hourFrames: // the night begins: who is where, to be there at dawn
		g.rollSet("O3.night", b2i(g.together("magda", "celestine")))
		g.rollSet("O10.night", b2i(g.together("magenta", "celestine")))
	case runStart: // dawn
		g.dawnChecks()
	}
}

// dawnChecks: the objectives that wait for a night: Magda and Madame (O3), Magenta and Madame (O10), and Lv2+'s
// longer word (O1).
func (g *Game) dawnChecks() {
	for _, c := range [...]struct{ id, a, says string }{{"O3", "magda", "magda"}, {"O10", "magenta", "celestine"}} {
		if g.questOpen(c.id) && g.rollGet(c.id+".night") == 1 && g.together(c.a, "celestine") {
			g.sayDone(g.questParty(c.says), objOf(c.id).Done)
			g.verbDone(evDawn, g.roomOfKey("celestine"), c.id)
		}
		g.rollSet(c.id+".night", 0)
	}
	if g.level >= 2 && g.questOpen("O1") && g.rollGet("O1.long") == 0 && day(g.clock) > g.rollGet("O1.start") && g.rollGet("O1.start") > 0 {
		g.rollSet("O1.long", 1) // it did not arrive in time: she sends a second, longer word
		g.verbs.held = ""
	}
}

// rollOpen: roll or pick the parts of the open objectives that need the hotel as it is.
func (g *Game) rollOpen() {
	v := &g.verbs
	if g.questOpen("O1") && g.rollGet("O1.start") == 0 {
		g.rollSet("O1.start", day(g.clock))
	}
	if g.questOpen("O3") && g.level >= 2 && g.rollGet("O3.stranger") == 0 {
		g.pickStranger()
	}
	if g.questOpen("O6") && g.rollGet("O6.room") == 0 {
		var ghosts []*Party
		for _, p := range g.parties {
			if p.Realm == Ghost && p.state == staying && p.Guests[0].Who.Key == "" && p.Guests[0].PassAt == 0 {
				ghosts = append(ghosts, p)
			}
		}
		if len(ghosts) > 0 {
			p := ghosts[g.roll("O6.pick", len(ghosts))]
			v.porter = p.Guests[0]
			g.rollSet("O6.room", p.Room+1)
		}
	}
	if g.questOpen("O9") && g.rollGet("O9.took") == 0 {
		if o := g.oldestParty(); o != nil && o != v.oldest {
			v.oldest = o
			if g.level >= 2 { // out until night
				o.Routine = tourist
			}
		}
	}
	if g.questOpen("O11") && g.rollGet("O11.ally") == 0 {
		var here []int
		for i, k := range allies {
			if p := g.questParty(k); p != nil && p.Realm == Ghost {
				here = append(here, i)
			}
		}
		if len(here) > 0 {
			g.rollSet("O11.ally", here[g.roll("O11.pick", len(here))]+1)
		}
	}
	if g.questOpen("O12") {
		g.roll("O12.at", 5)
	}
}

// pickStranger (O3, Lv2+): Madame's Room holds a human who is not Magda: one is brought if it is empty. It stays
// until Magda is in (verbPinned).
func (g *Game) pickStranger() {
	room, magda := g.roomOfKey("celestine"), g.questParty("magda")
	if room < 0 || magda == nil || !g.isOwned(room) {
		return
	}
	g.rollSet("O3.stranger", 1)
	if magda.Room == room {
		return
	}
	q := g.humanIn(room)
	if q == nil {
		if q = g.arrive(room); q == nil {
			return
		}
		q.Guests, q.Routine, q.Nights = q.Guests[:1], homebody, 3
		q.schedule(g.clock)
	}
	g.verbs.stranger = q
}

// verbWait: the objective can't be played yet and its row says so (Objective.Wait): O1 has no ghost in Magenta's Room.
func (g *Game) verbWait(id string) bool {
	if id != "O1" || !g.questOpen("O1") {
		return false
	}
	m := g.questParty("magenta")
	return m != nil && !slices.ContainsFunc(g.parties, func(p *Party) bool { return p.Realm == Ghost && p.Room == m.Room && p.state == staying })
}

// askTim (O5): the ghost asks for a Supply, rolled once; at Lv2+ a second one after the first is served.
func (g *Game) askTim() {
	p := g.questParty("tim")
	if p == nil || p.Realm != Ghost || p.state != staying || p.asking() || g.rollGet("O5.n") >= askCount(g.level) {
		return
	}
	k := (g.roll("O5.supply", len(supplies)) + g.rollGet("O5.n")) % len(supplies)
	p.asked, p.want = g.clock, supplies[k].ID
}

func askCount(level int) int { return 1 + b2i(level >= 2) }

// ghostWants: the Room's ghost asks for item, and it is the Request of a quest (only they ask).
func (g *Game) ghostWants(room int, item Unlock) bool {
	for _, p := range g.parties {
		if p.Occupies(room, Ghost) && p.asking() && p.state == staying && p.want == item && item != "" {
			return true
		}
	}
	return false
}

// serveGhost: SPACE at a ghost's door in ghost with its asked Supply in the hands: delivered, nothing paid.
func (g *Game) serveGhost(room int) bool {
	for _, p := range g.parties {
		if !p.Occupies(room, Ghost) || !p.asking() || p.want != g.hands || g.hands == "" {
			continue
		}
		p.asked, p.askEnded = 0, g.clock
		g.hands = ""
		n := g.rollGet("O5.n") + 1
		g.rollSet("O5.n", n)
		if n >= askCount(g.level) {
			g.verbDone(evServe, room, "O5")
		}
		return true
	}
	return false
}

// ghostDoor: SPACE at Room i's door in ghost does something of a quest: a ghost's Request, the wreath, the handprint.
func (g *Game) ghostDoor(i int) bool {
	return g.ghostWants(i, g.hands) || g.wreathAt(i) || g.handprintRoom() == i
}

// wreathAt: the wreath lies at Room i's door and Madame is in it.
func (g *Game) wreathAt(i int) bool {
	return g.questOpen("O8") && g.rollGet("O8.parcel") == i+1 && g.roomOfKey("celestine") == i
}

// ghostDeliver: what ghostDoor said happens.
func (g *Game) ghostDeliver(i int) {
	switch {
	case g.ghostWants(i, g.hands):
		g.serveGhost(i)
	case g.wreathAt(i):
		g.rollSet("O8.parcel", 0)
		g.sayDone(g.questParty("celestine"), objOf("O8").Done)
		g.verbDone(evDeliver, i, "O8")
	case g.handprintRoom() == i:
		g.verbDone(evDoor, i, "O4")
	}
}

// verbOpener: the Priest's opening line when the quest has one for this Room: K's retirement party (O6), then either
// Father's memory of the questline's Room.
func (g *Game) verbOpener(pr *priest) []string {
	room := pr.RoomIndex
	if room < 0 || g.quest.Beat < 1 {
		return nil
	}
	if !pr.strong && g.questOpen("O6") && g.porterRoom() == room {
		return []string{byTimes(fatherKParty, g.timesDone(4))}
	}
	n := g.timesDone(g.quest.Beat)
	if n == 0 || !slices.ContainsFunc(g.parties, func(p *Party) bool { return p.Room == room && slices.Contains(questCast, p.Guests[0].Who.Key) }) {
		return nil
	}
	if pr.strong {
		return []string{fatherMQuest[min(n-1, 1)]}
	}
	return []string{fatherKQuest}
}

// porterRoom: the Room of O6's porter, -1 before it is rolled.
func (g *Game) porterRoom() int { return g.rollGet("O6.room") - 1 }

// verbRitual: a Priest's Ritual landed. Father K's on the porter's Room is the retirement party: it passes on
// happily, and O6 is done.
func (g *Game) verbRitual(pr *priest) {
	if pr.strong || !g.questOpen("O6") || g.porterRoom() != pr.RoomIndex {
		return
	}
	p := g.porterParty()
	if p == nil {
		return
	}
	g.rollSet("O6.done", 1)
	for _, guest := range p.Guests {
		guest.PassAt, guest.Path = g.clock, nil
	}
	g.verbDone(evRitual, pr.RoomIndex, "O6")
}

// The quest rows' IDs (shop.go).
const (
	rowWreath Unlock = "quest-wreath"
	rowCrate  Unlock = "quest-crate"
	rowStorm  Unlock = "item-storm"
)

// placeNamed settles the named guest, solo, in the Room for a few nights (dev prints and tests); its Party, nil if the
// Room can't take it.
func (g *Game) placeNamed(room int, key string) *Party {
	var p *Party
	if i := slices.IndexFunc(namedGhosts, func(n namedGhost) bool { return n.Key == key }); i >= 0 {
		if p = g.arriveGhost(room); p == nil {
			return nil
		}
		p.Guests, p.Nights = p.Guests[:1], 3
		g.nameGhost(p.Guests[0], namedGhosts[i])
	} else if i := slices.IndexFunc(namedHumans, func(h namedHuman) bool { return h.Key == key }); i >= 0 {
		if p = g.arrive(room); p == nil {
			return nil
		}
		h, m := namedHumans[i], namedHumans[i].Members[0]
		p.Guests, p.Routine, p.Nights = p.Guests[:1], homebody, 3
		p.Guests[0].Who = entities.Identity{Name: m.Name, Age: m.Age, AgeNote: m.AgeNote, Job: h.Job, Lines: m.Lines, Quirk: h.Quirk, Key: h.Key}
		p.Guests[0].Character = g.dressed(m.Sprite)
		wear(p.Guests[0])
	} else {
		return nil
	}
	p.schedule(g.clock)
	g.settleIn(p)
	return p
}

// devCast: who -verb O<n> puts where ("key@room", a plain "human" or "ghost" a generic guest), as -grid 3x1 has the Rooms.
var devCast = map[string][]string{
	"O1": {"magenta@0", "ghost@0"}, "O2": {"magenta@0", "celestine@1"}, "O3": {"magda@0", "celestine@1"},
	"O5": {"tim@0"}, "O6": {"teo@0", "ghost@1"}, "O7": {"magda@0", "osvaldo@1"}, "O8": {"celestine@1"},
	"O9": {"tim@0", "human@1", "human@2"}, "O10": {"magenta@0", "celestine@1"}, "O11": {"magenta@0", "celestine@1", "tim@2"},
}

// DevVerb is main's -verb O<n>: a Run at that objective with its guests in, for prints. In ghost (-ghost) it shows the
// ghost side where it differs: the TELL Card (O1), Celestine's entry (O2), the handprint (O4). O4 starts at night with
// the Concierge in the beam's way (-at 14); O5 has Tim asking, the Supply in the hands; O8 opens the Terminal.
func (g *Game) DevVerb(id string) {
	if id == "" {
		return
	}
	beat, n := 0, 0
	for b := range beats {
		for _, o := range beats[b].Objs {
			if o.ID == id {
				beat, n = b+1, len(beats[b].Objs)
			}
		}
	}
	if beat == 0 {
		return
	}
	g.unlock(Mirror)
	g.roomOwned = nil
	g.arrivals, g.ghostArrivals = make([]arrival, len(g.hotel.Rooms)), make([]arrival, len(g.hotel.Rooms))
	g.wallet = [2]Num{max(g.wallet[Money], 2500), max(g.wallet[Ecto], 600)}
	g.quest = questRun{Beat: beat, Done: make([]bool, n), Rolls: map[string]int{}}
	g.verbs = verbState{beat: beat}
	last := len(g.hotel.Rooms) - 1
	for _, c := range devCast[id] {
		key, at, _ := strings.Cut(c, "@")
		room := min(int(at[0]-'0'), last)
		switch key {
		case "human":
			p := g.arrive(room)
			p.Guests, p.Routine, p.Nights = p.Guests[:1], homebody, 3
			p.schedule(g.clock)
			g.settleIn(p)
		case "ghost":
			p := g.arriveGhost(room)
			p.Guests, p.Nights = p.Guests[:1], 3
			p.schedule(g.clock)
			g.settleIn(p)
		default:
			g.placeNamed(room, key)
		}
	}
	plate := g.hotel.Rooms[0].Plate()
	at := func(r image.Rectangle) {
		c := r.Min.Add(r.Max).Div(2)
		g.player.X, g.player.Y = float64(c.X), float64(c.Y)
	}
	switch id {
	case "O1":
		g.verbs.held = "O1"
		at(plate)
		g.startCard(0)
	case "O2":
		for _, e := range []bookEntry{{nameTeoFull, namedHumans[1].Visit[0], 1, outCozy}, {"Agatha Fogg", notebookPassed[0], 1, outPassed}} {
			g.guestBook = append(g.guestBook, e)
		}
		g.openBook()
	case "O4":
		g.rollSet("O4.room", 1)
		g.rollSet("O4.door", 1)
		g.clock = 23 * hourFrames
		if g.ghost {
			g.rollSet("O4.lit", 1)
			at(g.hotel.Rooms[0].Mat())
			break
		}
		x0, x1, y := g.spiritLane(g.spiritRow())
		g.player.X, g.player.Y = (x0+x1)/2, y
		g.player.Anim.Facing = entities.FaceLeft
	case "O5":
		g.hands = SupplyTowel
		if p := g.questParty("tim"); p != nil {
			p.asked, p.want = g.clock, SupplyTowel
		}
		at(g.hotel.Rooms[0].Mat())
	case "O8":
		g.openTerminal()
		g.termTab = tabHotel
		g.termPick = slices.IndexFunc(tabItems(tabHotel), func(i int) bool { return shop[i].ID == rowWreath })
	}
	w, h := g.mapSize()
	g.cam.Snap(g.player.X, g.player.Y, w, h)
}
