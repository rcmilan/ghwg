package game

import (
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"maps"
	"math/rand/v2"
	"slices"

	"ghosthotel/entities"
	"ghosthotel/world"
)

// 08 W3 (P1-S): the save. Only a Game with persist set reads or writes it. It holds the prestige data and the
// last dawn checkpoint (08:00, every day), taken with nobody in the hotel: resuming restarts that morning, empty.
// After a prestige (saveGame) it holds no Run: Continue starts the next one fresh at the new level.
// Decoding is tolerant: a missing field takes its zero value, an unknown one is ignored; the storage key moves to
// v2 only on a breaking change. Every new run-state feature adds its fields to runState.

// saveData is what the save holds.
type saveData struct {
	Level int       `json:"level"`
	Bonus float64   `json:"bonus"`
	Run   *runState `json:"run,omitempty"`
	// Revealed: the named guests whose card was revealed (card.go); they recur across Runs.
	Revealed []string `json:"revealed,omitempty"`
	// Visits per named guest, across Runs (notebook.go): a prestige keeps them like the level.
	Visits map[string]int `json:"visits,omitempty"`
	// Quest: Magenta's circle across Runs (quest.go, 08 W5): kept by a prestige like the level.
	Quest *questRecord `json:"quest,omitempty"`
}

// upLevels: the saved levels as g.up. A save from before Bigger shelves went has 8, that upgrade the 7th: drop its slot.
// An older PantryStock in a save is ignored: the Pantry has no stock of its own.
func upLevels(s []int) (up [upgradeCount]int) {
	if len(s) > len(up) {
		s = slices.Delete(slices.Clone(s), 6, 7)
	}
	copy(up[:], s)
	return
}

// retired: the Veggie and Gourmet meals of an older save, gone with the one Food; restore drops them.
func retired(id Unlock) bool { return id == "supply-veggie" || id == "supply-gourmet" }

// runState: the dawn checkpoint.
type runState struct {
	Cols, Rows int         // the hotel's Room grid
	Seed       uint64      // the Run's rngs from here on, drawn at the checkpoint
	Day        int         // the checkpoint is its 08:00
	Rooms      []roomState // map order
	Corridors  []corridorState
	Keepers    []payroll         // housekeepers on the payroll
	FatherK    *payroll          // the retained Father K, nil if none
	Wallet     [2]Num            // Money, Ecto
	Earned     [2]Num            // lifetime this Run: opens store items
	RunEcto    Num               // Ectoplasm earned this Run, before the bonus
	Locked     []Unlock          // the unlocks still locked; nil: a fresh Run's
	Bought     map[Unlock][2]Num // unlocks bought, with Earned when each arrived
	Seen       []Unlock          // store items already seen: the Terminal's "!" skips them
	Stock      map[Unlock]int
	Orders     []orderLine // paid for and still on a courier: they arrive at the checkpoint
	Up         []int       // the leveled upgrades (upgrade.go); an older save has none: all 0, or 8 (upLevels)
	Book       []bookEntry // the notebook (notebook.go)
	Met        []string    // the named guests that have booked this Run, sorted
	Played     int         `json:",omitempty"` // real frames played to the checkpoint (the run timer)
	Quest      *questRun   `json:",omitempty"` // this Run's questline (08 W5)
	Items      *itemRun    `json:",omitempty"` // where the items are this Run (08 W5)
}

type roomState struct {
	Tier, Beds string // its Room Type (a change still waiting is applied)
	Owned      bool
	Mirror     bool // its in-Room Mirror
	Mood       int
	Dirt       []dirtSpot
	DirtN      int
	Taps       [taskCount]Num
}

type dirtSpot struct {
	X, Y   int
	Spirit bool
	Size   int
}

type corridorState struct {
	Floor, Col int
	West, Open bool
}

// payroll: a member of staff, its payday DueIn frames from the checkpoint.
type payroll struct {
	DueIn  int
	Unpaid bool
}

type orderLine struct {
	Item    Unlock
	Room, N int
	Beds    string
}

// The byte store behind the save, one per platform (save_js.go, save_other.go); the tests swap them. Reading a
// save that does not exist gives nil bytes and no error.
var (
	readSave  = platformRead
	writeSave = platformWrite
)

// Persist makes this Game read and write the save: main's plain start only.
func (g *Game) Persist() { g.persist = true }

// saveGame writes the save right after a prestige (level and bonus updated, the Run over): no Run in it.
func (g *Game) saveGame() {
	g.write(&saveData{Level: g.level, Bonus: g.bonus, Revealed: g.revealedList(), Visits: g.visits, Quest: &g.questRec})
}

// saveDawn writes the dawn checkpoint.
func (g *Game) saveDawn() {
	if g.persist {
		g.write(&saveData{Level: g.level, Bonus: g.bonus, Run: g.checkpoint(), Revealed: g.revealedList(), Visits: g.visits, Quest: &g.questRec})
	}
}

func (g *Game) write(s *saveData) {
	if !g.persist {
		return
	}
	b, err := json.Marshal(s)
	if err == nil {
		err = writeSave(b)
	}
	if err != nil {
		g.notice("COULD NOT SAVE")
	}
}

// loadSave reads the save: nil if there is none, or it is unreadable (with a notice, never a crash).
func (g *Game) loadSave() *saveData {
	if !g.persist {
		return nil
	}
	b, err := readSave()
	if err == nil && len(b) == 0 {
		return nil
	}
	var s *saveData
	if err == nil {
		s, err = decodeSave(b)
	}
	if err != nil {
		g.notice("SAVE UNREADABLE - NEW GAME")
		return nil
	}
	return s
}

func decodeSave(b []byte) (s *saveData, err error) {
	defer func() { // a bad save never panics the game
		if r := recover(); r != nil {
			s, err = nil, fmt.Errorf("save: %v", r)
		}
	}()
	s = &saveData{}
	if err = json.Unmarshal(b, s); err != nil {
		return nil, err
	}
	return s, nil
}

// checkpoint: the Run as it stands, for the dawn save.
func (g *Game) checkpoint() *runState {
	cols, rows := g.hotel.Grid()
	r := &runState{Cols: cols, Rows: rows, Seed: g.rng.Uint64(), Day: day(g.clock), Wallet: g.wallet, Earned: g.earned, RunEcto: g.runEcto,
		Bought: g.boughtAt, Stock: g.stock, Locked: []Unlock{}, Up: g.up[:], Book: slices.Clone(g.guestBook), Met: slices.Sorted(maps.Keys(g.met)), Played: g.played, Quest: &g.quest, Items: &g.items}
	for _, id := range slices.Sorted(maps.Keys(g.locked)) {
		if g.locked[id] {
			r.Locked = append(r.Locked, id)
		}
	}
	r.Seen = slices.Sorted(maps.Keys(g.seen))
	waiting := map[int]world.RoomType{}
	for _, c := range g.changes {
		if c.reel == 0 {
			waiting[c.room] = c.to
		}
	}
	for i, room := range g.hotel.Rooms {
		t := room.Type
		if w, ok := waiting[i]; ok {
			t = w
		}
		rs := roomState{Tier: t.Name, Beds: t.Beds, Owned: g.isOwned(i), Mirror: !room.Mirror.Empty(), Mood: g.roomMood(i)}
		if i < len(g.dirt) {
			for _, s := range g.dirt[i] {
				rs.Dirt = append(rs.Dirt, dirtSpot{s.cell.X, s.cell.Y, s.spirit, s.size})
			}
		}
		if i < len(g.dirtN) {
			rs.DirtN = g.dirtN[i]
		}
		if i < len(g.taps) {
			rs.Taps = g.taps[i]
		}
		r.Rooms = append(r.Rooms, rs)
	}
	for _, m := range g.hotel.Corridors {
		r.Corridors = append(r.Corridors, corridorState{m.Floor, m.Col, m.West, m.Open})
	}
	for _, k := range g.keepers {
		if k.job != quitting {
			r.Keepers = append(r.Keepers, payroll{max(0, k.due-g.clock), k.unpaid})
		}
	}
	for _, pr := range g.priests {
		if pr.retained {
			r.FatherK = &payroll{DueIn: max(0, pr.due-g.clock)}
		}
	}
	for _, c := range g.couriers {
		for _, d := range c.drops {
			for _, l := range d.lines {
				r.Orders = append(r.Orders, orderLine{shop[l.item].ID, l.room, l.n, l.beds})
			}
		}
	}
	return r
}

// resume applies the save from the title. False if its Run does not fit this hotel: the game is then a fresh Run
// again, with a notice, and the title goes on as without a save.
func (g *Game) resume(s *saveData) bool {
	g.level, g.bonus = s.Level, max(1, s.Bonus)
	for _, k := range s.Revealed {
		if g.revealedKeys == nil {
			g.revealedKeys = map[string]bool{}
		}
		g.revealedKeys[k] = true
	}
	g.visits = s.Visits
	if s.Quest != nil {
		g.questRec = *s.Quest
	}
	if s.Run != nil {
		if err := g.restore(s.Run); err != nil {
			g.startRun()
			g.notice("SAVE UNREADABLE - NEW GAME")
			return false
		}
	}
	g.intro, g.ghost, g.state = nil, false, StatePlay
	return true
}

// restore sets the Run to the checkpoint r, over the fresh Run startRun made.
func (g *Game) restore(r *runState) error {
	if r.Cols < 1 || r.Rows < 1 || r.Cols > 16 || r.Rows > 16 || len(r.Rooms) != r.Cols*r.Rows {
		return errors.New("save: bad grid")
	}
	if c, k := g.hotel.Grid(); c > r.Cols || k > r.Rows {
		return errors.New("save: grid smaller than the start's")
	}
	for { // the hotel grows the way the Run did (any order ends in the same hotel)
		c, k := g.hotel.Grid()
		if c == r.Cols && k == r.Rows {
			break
		}
		if _, err := g.grow(c < r.Cols); err != nil {
			return err
		}
	}
	n := len(r.Rooms)
	g.rng, g.chat, g.words = rand.New(rand.NewPCG(r.Seed, r.Seed)), rand.New(rand.NewPCG(r.Seed, r.Seed+1)), rand.New(rand.NewPCG(r.Seed, r.Seed+2))
	g.seedIdent(r.Seed)
	g.guestBook, g.met, g.leftAt, g.played = slices.Clone(r.Book), nil, nil, r.Played
	if r.Quest != nil {
		g.quest = *r.Quest
	}
	if r.Items != nil {
		g.items = *r.Items
	}
	for _, k := range r.Met {
		if g.met == nil {
			g.met = map[string]bool{}
		}
		g.met[k] = true
	}
	g.up = upLevels(r.Up)
	var ms []world.CorridorMirror
	for _, m := range r.Corridors {
		ms = append(ms, world.CorridorMirror{Floor: m.Floor, Col: m.Col, West: m.West, Open: m.Open})
	}
	g.hotel.SetCorridorMirrors(ms) // before the Rooms land, which hang them
	g.clock = (max(1, r.Day)-1)*dayFrames + runStart
	g.wallet, g.earned, g.runEcto, g.recent = r.Wallet, r.Earned, r.RunEcto, nil
	if r.Locked != nil {
		g.locked = map[Unlock]bool{}
		for _, id := range r.Locked {
			g.locked[id] = !retired(id)
		}
	}
	g.openPantries() // the growths above ran before the locks: stand the shelves now
	g.boughtAt, g.stock = r.Bought, r.Stock
	for id := range g.boughtAt {
		if retired(id) {
			delete(g.boughtAt, id)
		}
	}
	for id := range g.stock {
		if retired(id) {
			delete(g.stock, id)
		}
	}
	if g.stock == nil {
		g.stock = map[Unlock]int{}
	}
	g.seen = nil
	if len(r.Seen) > 0 {
		g.seen = map[Unlock]bool{}
		for _, id := range r.Seen {
			g.seen[id] = !retired(id)
		}
	}
	g.roomOwned, g.mood, g.dirt, g.dirtN, g.taps = make([]bool, n), make([]int, n), make([][]spot, n), make([]int, n), make([][taskCount]Num, n)
	for i, rs := range r.Rooms {
		t, ok := world.TypeIn(g.types, rs.Tier, rs.Beds)
		if !ok {
			t, _ = world.TypeOf(g.types, "Single")
		}
		g.hotel.Land(i, t, rs.Mirror)
		g.roomOwned[i], g.mood[i], g.dirtN[i], g.taps[i] = rs.Owned, max(moodMin, min(moodMax, rs.Mood)), rs.DirtN, rs.Taps
		for _, s := range rs.Dirt {
			g.dirt[i] = append(g.dirt[i], spot{image.Pt(s.X, s.Y), s.Spirit, s.Size})
		}
	}
	g.shades = make([]shades, n)
	g.fog, g.mirrorSlots, g.hushed = nil, nil, map[int]bool{}
	g.arrivals, g.ghostArrivals = g.firstArrivals(), g.firstArrivals()
	g.floorImg, g.stains, g.spill = nil, nil, nil
	g.keepers, g.priests = nil, nil
	for _, p := range r.Keepers {
		g.keepers = append(g.keepers, &housekeeper{Guest: entities.NewGuest(g.kitchenSpot(), keeperName), due: g.clock + p.DueIn, unpaid: p.Unpaid})
	}
	if r.FatherK != nil {
		g.retainK()
		g.priests[0].due = g.clock + r.FatherK.DueIn
	}
	for _, o := range r.Orders {
		if i := slices.IndexFunc(shop, func(it Item) bool { return it.ID == o.Item }); i >= 0 && o.Room < n { // -1: to the stock (Supplies, unlocks, upgrade levels)
			g.receive(cartLine{item: i, room: o.Room, n: o.N, beds: o.Beds})
		}
	}
	g.player = entities.NewPlayer(g.hotel.ConciergeStart)
	w, h := g.mapSize()
	g.cam.Snap(g.player.X, g.player.Y, w, h)
	return nil
}
