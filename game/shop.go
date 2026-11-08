package game

import (
	"slices"
	"strings"
)

// The Terminal is a web store (D180, redesigned 2026-11-08): tabs of items, a cart, a checkout.
type Tab int

const (
	tabSupplies Tab = iota // restock: into the hotel's stock
	tabHotel               // hotel-wide unlocks
	tabServices            // for one Room, picked when added to the cart
	tabCart
	tabs
)

var tabNames = [tabs]string{"SUPPLIES", "HOTEL", "SERVICES", "CART"}

// Item is one product of the store.
type Item struct {
	ID     Unlock
	Tab    Tab
	Name   string
	Blurb  string // one cartLine on the detail pane
	Icon   string // assets/sprites/<Icon>.png, or a character whose head is the icon (Head)
	Head   bool
	Cost   Cost
	Pack   int      // Supplies: units into stock per buy
	Needs  []Unlock // owned first, then
	Earned Cost     // earned since the last of Needs was bought. No Needs and no Earned: on sale from the start
	// Services only: why the Room can't take it ("" if it can), and the paid-for delivery.
	Refuse func(g *Game, room int) string
	Send   func(g *Game, room int)
}

// The Services aren't unlocks; their IDs only name them.
const (
	rowFatherK Unlock = "father-k"
	rowFatherM Unlock = "father-m"
	rowShadow  Unlock = "shadow"
)

func noPriest(g *Game, room int) string {
	if g.priestOn(room) {
		return "A PRIEST IS ALREADY ON ROOM " + g.hotel.Rooms[room].Number
	}
	return ""
}

// shop is the catalog, in its order. Prices, packs and thresholds are placeholders.
var shop = []Item{
	{ID: SupplyWater, Tab: tabSupplies, Name: "Water", Blurb: "A case of bottles", Icon: "icon_water", Cost: Cost{Money: 10}, Pack: 5},
	{ID: SupplyFood, Tab: tabSupplies, Name: "Food", Blurb: "Room service meals", Icon: "food_4", Cost: Cost{Money: 20}, Pack: 5},
	{ID: SupplyTowel, Tab: tabSupplies, Name: "Towels", Blurb: "Fresh and folded", Icon: "icon_towel", Cost: Cost{Money: 15}, Pack: 5},

	{ID: Mirror, Tab: tabHotel, Name: "The Mirror", Blurb: "Step into the other side", Icon: "mirror_standing", Cost: Cost{Money: 10}},
	{ID: Hex, Tab: tabHotel, Name: "Hex", Blurb: "Curse a Room from any wall", Icon: "painting_scary_a", Cost: Cost{Money: 25},
		Needs: []Unlock{Mirror}, Earned: Cost{Spook: 10}},
	{ID: Housekeep, Tab: tabHotel, Name: "Housekeeping", Blurb: "Clean empty Rooms", Icon: "icon_towel", Cost: Cost{Money: 50, Spook: 20},
		Needs: []Unlock{Mirror}, Earned: Cost{Money: 100, Spook: 30}},
	{ID: RoomLock, Tab: tabHotel, Name: "Room lock", Blurb: "Nobody leaves early", Icon: "lock_red", Cost: Cost{Money: 60, Spook: 40},
		Needs: []Unlock{Hex}, Earned: Cost{Spook: 100}},
	{ID: Consume, Tab: tabHotel, Name: "Consume", Blurb: "...", Icon: "painting_scary_b", Cost: Cost{Money: 100, Spook: 50},
		Needs: []Unlock{RoomLock}, Earned: Cost{Spook: 200}},
	{ID: ShiftAnywhere, Tab: tabHotel, Name: "Shift anywhere", Blurb: "Every mirror is a door", Icon: "vanity", Cost: Cost{Money: 500, Spook: 500},
		Needs: []Unlock{Mirror}, Earned: Cost{Money: 1000, Spook: 1000}},

	{ID: rowFatherK, Tab: tabServices, Name: "Father K", Blurb: "Halves a Room's Spook", Icon: fatherK.name, Head: true, Cost: Cost{Money: priestWeak},
		Needs: []Unlock{Mirror}, Earned: Cost{Money: 50}, Refuse: noPriest, Send: func(g *Game, room int) { g.sendPriest(room, false) }},
	{ID: rowFatherM, Tab: tabServices, Name: "Father M", Blurb: "Clears it, banishes a Shadow", Icon: fatherM.name, Head: true, Cost: Cost{Money: priestStrong},
		Needs: []Unlock{Mirror}, Earned: Cost{Money: 150}, Refuse: noPriest, Send: func(g *Game, room int) { g.sendPriest(room, true) }},
	{ID: rowShadow, Tab: tabServices, Name: "Shadow Person", Blurb: "Scares on its own", Icon: "Adam", Head: true, Cost: Cost{Spook: shadowPrice},
		Needs: []Unlock{Mirror}, Earned: Cost{Spook: 50}, Send: func(g *Game, room int) { g.placeShadow(room) },
		Refuse: func(g *Game, room int) string {
			if g.shaded(room) {
				return "ROOM " + g.hotel.Rooms[room].Number + " HAS A SHADOW ALREADY"
			}
			return ""
		}},
}

// cartLine is one cart entry: n of a Supply, an unlock, or a Service for a Room (else room is -1).
type cartLine struct{ item, room, n int }

func (l cartLine) cost() Cost {
	c := shop[l.item].Cost
	return Cost{c[Money] * Num(l.n), c[Spook] * Num(l.n)}
}

// onSale: its Needs are owned and, since the last of them was bought, the hotel earned its Earned.
// Buying never opens another item by itself (2026-11-08).
func (g *Game) onSale(i int) bool {
	it := shop[i]
	var since [2]Num
	for _, u := range it.Needs {
		if !g.unlocked(u) {
			return false
		}
		b := g.boughtAt[u]
		since = [2]Num{max(since[0], b[0]), max(since[1], b[1])}
	}
	return g.earned[Money]-since[Money] >= it.Earned[Money] && g.earned[Spook]-since[Spook] >= it.Earned[Spook]
}

// owned: an unlock already bought, or already in the cart.
func (g *Game) owned(i int) bool {
	it := shop[i]
	return it.Tab == tabHotel && (g.unlocked(it.ID) || slices.ContainsFunc(g.cart, func(l cartLine) bool { return l.item == i }))
}

// addToCart puts item i in the cart, for the Room of a Service: "" or why not.
func (g *Game) addToCart(i, room int) string {
	it := shop[i]
	switch {
	case !g.onSale(i):
		return "COMING SOON"
	case it.Tab == tabHotel && g.unlocked(it.ID):
		return "ALREADY OWNED"
	case g.owned(i):
		return "ALREADY IN THE CART"
	case !g.affords(i):
		return "NOT ENOUGH IN THE WALLET"
	case it.Tab == tabServices:
		if why := it.Refuse(g, room); why != "" {
			return why
		}
		if slices.ContainsFunc(g.cart, func(l cartLine) bool { return l.room == room && sameKind(l.item, i) }) {
			return "ALREADY IN THE CART FOR ROOM " + g.hotel.Rooms[room].Number
		}
		g.cart = append(g.cart, cartLine{i, room, 1})
		return ""
	}
	if k := slices.IndexFunc(g.cart, func(l cartLine) bool { return l.item == i }); k >= 0 {
		g.cart[k].n++
		return ""
	}
	g.cart = append(g.cart, cartLine{i, -1, 1})
	return ""
}

// affords: the cart plus one more of item i fits the wallet (2026-11-08: no adding what you can't pay).
func (g *Game) affords(i int) bool {
	t, c := g.cartTotal(), shop[i].Cost
	return g.afford(Cost{t[Money] + c[Money], t[Spook] + c[Spook]})
}

// sameKind: both Priests (one per Room), or the same Service.
func sameKind(a, b int) bool {
	priest := func(i int) bool { return shop[i].ID == rowFatherK || shop[i].ID == rowFatherM }
	return a == b || priest(a) && priest(b)
}

// cartTotal: what checkout charges.
func (g *Game) cartTotal() Cost {
	var t Cost
	for _, l := range g.cart {
		c := l.cost()
		t[Money] += c[Money]
		t[Spook] += c[Spook]
	}
	return t
}

// removeLine takes one unit off cart cartLine k, the cartLine when it's the last.
func (g *Game) removeLine(k int) {
	if g.cart[k].n--; g.cart[k].n <= 0 {
		g.cart = slices.Delete(g.cart, k, k+1)
	}
}

// placeOrder pays the whole cart, or nothing, and ships it: "" or why not.
func (g *Game) placeOrder() string {
	if len(g.cart) == 0 {
		return "THE CART IS EMPTY"
	}
	t := g.cartTotal()
	if !g.pay(t) {
		return "NOT ENOUGH: " + price(t)
	}
	order := g.cart
	g.cart = nil
	g.ship(order)
	return ""
}

// ship sends a paid order on its way. E3 stand-in: everything arrives at once. E8's courier replaces it
// (Priests still walk in on their own).
func (g *Game) ship(order []cartLine) {
	for _, l := range order {
		g.receive(l)
	}
}

// receive: an order cartLine arrives. A Supply goes into stock, an unlock is owned, a Service starts.
func (g *Game) receive(l cartLine) {
	it := shop[l.item]
	switch it.Tab {
	case tabSupplies:
		if g.stock == nil {
			g.stock = map[Unlock]int{}
		}
		g.stock[it.ID] += it.Pack * l.n
	case tabHotel:
		g.unlock(it.ID)
		if g.boughtAt == nil {
			g.boughtAt = map[Unlock][2]Num{}
		}
		g.boughtAt[it.ID] = g.earned
	case tabServices:
		it.Send(g, l.room)
	}
}

// price: "$40", "80 SPOOK" or both.
func price(c Cost) string {
	var p []string
	if c[Money] > 0 {
		p = append(p, "$"+fmtNum(c[Money]))
	}
	if c[Spook] > 0 {
		p = append(p, fmtNum(c[Spook])+" SPOOK")
	}
	return strings.Join(p, " + ")
}
