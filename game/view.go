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
	Title     bool // the title screen is up
	Ghost     bool // the hotel is in the ghost dimension
	Shifting  bool // the Mirror's transition is playing (D170): SPACE waits
	Clock     string
	Day       int
	Money     Num
	Spook     Num
	Rooms     []RoomView    // in the Terminal's order
	Guests    []image.Point // every guest's cell not hidden by the Fog (none is, in the ghost dimension)
	Concierge image.Point   // world px
	Terminal  TerminalView
	Shop      []int          // the store items that can go in the cart now, by index into shop (D180)
	Stock     map[Unlock]int // Supplies in the stock area, by ID, as its spots show them (D200)
	Courier   bool           // a courier is on his way with an order
	Hotel     *world.Hotel   // the map as drawn: walls, doors, anchors. Read it, never write it.
}

type RoomView struct {
	Number, Type, Beds string
	Vacant             bool
	Capacity, Rate     int
	Closed             bool // at the Spook limit: no new Party (D147)
	Priest             bool // one is on his way or at work
	Shadow             bool // a Shadow Person lives in it (D169), shown on the Terminal, or one is on its way
	Spook              int  // its Room Spook, read off its colors (D162)
	Away               bool // Occupied, but its Party isn't inside: the player saw it walk out
	Asking             bool // its Guests want service: the "!!" balloon, seen from anywhere (D163)
	// Seen only in the ghost dimension: its Guests glow with Fear.
	Fear int // its most frightened Guest in sight
}

type TerminalView struct {
	Open    bool
	Message string
	Cursor  int  // the Room a Service is for
	Tab     Tab  // the store's open tab
	Pick    int  // the picked row in the tab; in the cart, len(cart) is the checkout
	Placing bool // a Service waits for its Room
	Cart    int  // units in the cart
	Lines   int  // lines in the cart: the checkout row's Pick
}

func (g *Game) view() View {
	v := View{
		Title: g.state == StateTitle, Ghost: g.ghost, Shifting: g.shifting != nil,
		Clock: clockText(g.clock), Day: day(g.clock), Money: g.wallet[Money], Spook: g.wallet[Spook],
		Concierge: image.Pt(int(g.player.X), int(g.player.Y)), Hotel: g.hotel,
		Terminal: g.termView(),
		Shop:     g.forSale(),
		Stock:    maps.Clone(g.stock),
		Courier:  len(g.couriers) > 0,
	}
	for i, r := range g.hotel.Rooms {
		rv := RoomView{Number: r.Number, Type: r.Type.Name, Beds: r.Type.Beds, Vacant: g.vacant(i), Capacity: len(r.Stays), Rate: r.Type.Rate,
			Closed: g.closed(i), Priest: g.priestOn(i), Shadow: g.shaded(i) || g.coming(rowShadow, i), Spook: g.roomSpook[i]}
		rv.Asking = slices.ContainsFunc(g.parties, func(p *Party) bool { return p.Occupies(i) && p.asking() })
		rv.Away = !rv.Vacant && !slices.ContainsFunc(g.guests(), func(guest *entities.Guest) bool {
			return guest.RoomIndex == i && image.Pt(int(guest.X), int(guest.Y)).In(r.Shell())
		})
		if g.ghost {
			for _, guest := range g.guests() {
				if guest.RoomIndex == i {
					rv.Fear = max(rv.Fear, guest.Fear)
				}
			}
		}
		v.Rooms = append(v.Rooms, rv)
	}
	for _, guest := range g.guests() {
		if at := image.Pt(int(guest.X), int(guest.Y)); g.fogAt(at) < 1 {
			v.Guests = append(v.Guests, at.Div(world.TileSize))
		}
	}
	return v
}

func (g *Game) termView() TerminalView {
	t := TerminalView{Open: g.state == StateTerminal, Message: g.message, Cursor: g.termCursor, Tab: g.termTab, Pick: g.termPick, Placing: g.placing >= 0}
	for _, l := range g.cart {
		t.Cart += l.n
	}
	t.Lines = len(g.cart)
	return t
}

// forSale: the items on sale that aren't owned unlocks (bought, on their way or in the cart).
func (g *Game) forSale() []int {
	var rows []int
	for i := range shop {
		if g.onSale(i) && !g.owned(i) {
			rows = append(rows, i)
		}
	}
	return rows
}
