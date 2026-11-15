package game

import (
	"image"
	"maps"
	"slices"

	"ghosthotel/entities"
	"ghosthotel/world"
)

// View is what a player sees, nothing more (D105): a Strategy decides from it and the Pilot steers by it.
type View struct {
	Title        bool // the title screen is up
	Ghost        bool // the hotel is in the ghost dimension
	Shifting     bool // the Mirror's transition is playing (D170): SPACE waits
	Clock        string
	Day          int
	Money        Num
	Ecto         Num
	Rooms        []RoomView    // in the Terminal's order
	Guests       []image.Point // every guest's cell not hidden by the Fog (none is, in the ghost dimension)
	Concierge    image.Point   // world px
	Terminal     TerminalView
	Shop         []int          // the store items that can go in the cart now, by index into shop (D180)
	Stock        map[Unlock]int // Supplies in the stock area, by ID, as its spots show them (D200)
	MirrorLocked bool           // the Mirror is covered: SPACE at it shifts nothing
	Courier      bool           // a courier is on his way with an order
	Staff        int            // the housekeepers in the hotel
	Hands        Unlock         // the supply over the Concierge's head, "" for none (E5)
	Beds         []string       // the bed layouts a Room can change to: the unlocked ones (K1)
	RoomMirror   bool           // the in-Room Mirror is bought
	Hotel        *world.Hotel   // the map as drawn: walls, doors, anchors. Read it, never write it.
	Talk         TalkView       // the talk, open or not (07 X2)
	Matchmaking  bool           // owned: the housekeepers fix bad pairs (07 X2)
}

type RoomView struct {
	Number, Type, Beds string
	Vacant             bool
	Capacity, Rate     int
	Boarded            bool          // not owned yet: the boarded door, no bookings (07 B1)
	Changing           bool          // a change of type is waiting on it or rolling: no bookings (K1)
	Mirror             bool          // it has its own Mirror
	Closed             bool          // at Mood -100: no new Party (D147)
	Priest             bool          // one is on his way or at work
	Shadow             bool          // a Shadow Person lives in it (D169), shown on the Terminal, or one is on its way
	Mood               int           // its Room Mood, read off its colors (D162)
	Away               bool          // Occupied, but its Party isn't inside: the player saw it walk out
	Asking             bool          // its Guests want service: the balloon, seen from anywhere (D163)
	Wants              Unlock        // the supply in that balloon (E5)
	Dirt               []image.Point // its normal dirt spots, the ones to clean in real, world px (dirt.go)
	Spirit             []image.Point // its spiritual ones, to clean in ghost
	// Seen only in the ghost dimension: its Guests glow with Fear.
	Fear int  // its most frightened Guest in sight
	Bad  bool // a bad pair, red in the hotel view (07 X2)
	Fix  Num  // a bad pair: the cheapest sure offer to move its Party, as its talk shows it (0: none to make, or its guest snubbed)
}

type TerminalView struct {
	Open    bool
	View    bool // the hotel view is open
	Message string
	Cursor  int    // the Room a Service is for
	Tab     Tab    // the store's open tab
	Pick    int    // the picked row in the tab; in the cart, len(cart) is the checkout
	Depth   int    // how far into the hotel view: 1 the Rooms, 2 a Room's actions, 3 its beds
	Beds    string // the bed layout picked for a change of type
	Rows    []int  // in the hotel view: the Room's actions, shop indexes in order (hotelview.go's roomRows)
	Cart    int    // units in the cart
	Lines   int    // lines in the cart: the checkout row's Pick
}

func (g *Game) view() View {
	v := View{
		Title: g.state == StateTitle, Ghost: g.ghost, Shifting: g.shifting != nil,
		Clock: clockText(g.clock), Day: day(g.clock), Money: g.wallet[Money], Ecto: g.wallet[Ecto],
		Concierge: image.Pt(int(g.player.X), int(g.player.Y)), Hotel: g.hotel,
		Terminal:     g.termView(),
		Shop:         g.forSale(),
		Stock:        maps.Clone(g.stock),
		Courier:      len(g.couriers) > 0,
		Staff:        g.staffed(),
		Hands:        g.hands,
		MirrorLocked: !g.unlocked(Mirror),
		RoomMirror:   g.unlocked(RoomMirror),
		Talk:         g.talkView(),
		Matchmaking:  g.unlocked(Matchmaking),
	}
	for k, b := range bedKinds {
		if g.bedOpen(k) {
			v.Beds = append(v.Beds, b.Beds)
		}
	}
	for i, r := range g.hotel.Rooms {
		rv := RoomView{Number: r.Number, Type: r.Type.Name, Beds: r.Type.Beds, Vacant: g.vacant(i, Human), Capacity: len(r.Stays), Rate: r.Type.Rate,
			Boarded: !g.isOwned(i), Changing: g.changing(i), Mirror: !r.Mirror.Empty(), Closed: g.closed(i), Priest: g.priestOn(i), Shadow: g.shaded(i), Mood: g.roomMood(i), Bad: g.pairing(i) == badPair}
		for _, s := range g.spots(i) {
			at := s.cell.Mul(world.TileSize).Add(image.Pt(world.TileSize/2, world.TileSize/2))
			if s.spirit {
				rv.Spirit = append(rv.Spirit, at)
			} else {
				rv.Dirt = append(rv.Dirt, at)
			}
		}
		for _, p := range g.parties {
			if p.Occupies(i, Human) && p.asking() {
				rv.Asking, rv.Wants = true, p.want
			}
		}
		rv.Away = !rv.Vacant && !slices.ContainsFunc(g.guestsIn(Human), func(guest *entities.Guest) bool {
			return guest.RoomIndex == i && image.Pt(int(guest.X), int(guest.Y)).In(r.Shell())
		})
		if g.ghost {
			for _, guest := range g.guestsIn(Human) {
				if guest.RoomIndex == i {
					rv.Fear = max(rv.Fear, guest.Fear)
				}
			}
		}
		if p := g.humanIn(i); rv.Bad && p != nil && g.clock >= p.snubbed { // snubbed: no talk to make (X2)
			_, _, rv.Fix, _ = g.sureOffer(p)
		}
		v.Rooms = append(v.Rooms, rv)
	}
	for _, guest := range g.visible() {
		if at := image.Pt(int(guest.X), int(guest.Y)); g.fogAt(at) < 1 {
			v.Guests = append(v.Guests, g.hotel.Map.Cell(at))
		}
	}
	return v
}

func (g *Game) termView() TerminalView {
	t := TerminalView{Open: g.state == StateTerminal, View: g.state == StateView, Message: g.refused, Cursor: g.termCursor, Tab: g.termTab, Pick: g.termPick, Depth: g.termDepth, Beds: bedKinds[g.termBeds].Beds}
	if t.View && g.termCursor < len(g.hotel.Rooms) {
		t.Rows = g.roomRows(g.termCursor)
	}
	for _, l := range g.cart {
		t.Cart += l.n
	}
	t.Lines = len(g.cart)
	return t
}

// forSale: the items on sale that aren't owned unlocks (bought, on their way or in the cart), nor gone (terminal.go).
func (g *Game) forSale() []int {
	var rows []int
	for i := range shop {
		if g.onSale(i) && !g.owned(i) && !g.gone(i) {
			rows = append(rows, i)
		}
	}
	return rows
}

// refuse notes why the last SPACE did nothing ("" when it did): the View's Message, by which a Strategy knows
// a buy was turned down. It is never drawn (2026-11-14: no system text; the store's own pane says "CAN'T
// AFFORD", and a covered Mirror looks covered).
func (g *Game) refuse(why string) { g.refused = why }
