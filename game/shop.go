package game

import (
	"slices"
	"strings"

	"ghosthotel/entities"
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
)

func noPriest(g *Game, room int) string {
	if g.priestOn(room) {
		return "A PRIEST IS ALREADY ON ROOM " + g.hotel.Rooms[room].Number
	}
	return ""
}

// shop is the catalog, in its order. Prices, packs and thresholds are placeholders.
var shop = []Item{
	{ID: SupplyWater, Tab: tabSupplies, Name: "Water", Blurb: "Bottled at the source. Do not ask which.", Icon: "icon_water", Cost: Cost{Money: 10}, Pack: 5},
	{ID: SupplyFood, Tab: tabSupplies, Name: "Food", Blurb: "Hot meals, mostly from this dimension.", Icon: "food_4", Cost: Cost{Money: 20}, Pack: 5},
	{ID: SupplyTowel, Tab: tabSupplies, Name: "Towels", Blurb: "Fluffy. Stains come out. Most stains.", Icon: "icon_towel", Cost: Cost{Money: 15}, Pack: 5},

	{ID: Mirror, Tab: tabHotel, Name: "The Mirror", Blurb: "Totally not haunted. Ignore the knocking.", Icon: "mirror_standing", Cost: Cost{Money: 10}},
	// Housekeeping (Money 50 + Ecto 20, "Cleans empty Rooms. Asks no questions.") returns with auto-clean:
	// manual cleaning is free from the start (E6 close-out, 2026-11-08).
	{ID: RoomLock, Tab: tabHotel, Name: "Room lock", Blurb: "Checkout is whenever we say it is.", Icon: "lock_red", Cost: Cost{Money: 60, Ecto: 40},
		Needs: []Unlock{Mirror}, Earned: Cost{Ecto: 100}},
	{ID: Consume, Tab: tabHotel, Name: "Consume", Blurb: "Guests go in. Something else comes out.", Icon: "painting_scary_b", Cost: Cost{Money: 100, Ecto: 50},
		Needs: []Unlock{RoomLock}, Earned: Cost{Ecto: 200}},
	{ID: ShiftAnywhere, Tab: tabHotel, Name: "Shift anywhere", Blurb: "My husband shifted to another dimension. 5/5", Icon: "vanity", Cost: Cost{Money: 500, Ecto: 500},
		Needs: []Unlock{Mirror}, Earned: Cost{Money: 1000, Ecto: 1000}},

	{ID: rowFatherK, Tab: tabServices, Name: "Father K", Blurb: "Halves a Room's dread. Paid by the hour.", Icon: fatherK.name, Head: true, Cost: Cost{Money: priestWeak},
		Needs: []Unlock{Mirror}, Earned: Cost{Money: 50}, Refuse: noPriest, Send: func(g *Game, room int) { g.sendPriest(room, false) }},
	{ID: rowFatherM, Tab: tabServices, Name: "Father M", Blurb: "Clears a Room, banishes a Shadow. Cash only.", Icon: fatherM.name, Head: true, Cost: Cost{Money: priestStrong},
		Needs: []Unlock{Mirror}, Earned: Cost{Money: 150}, Refuse: noPriest, Send: func(g *Game, room int) { g.sendPriest(room, true) }},
}

// cartLine is one cart entry: n of a Supply, an unlock, or a Service for a Room (else room is -1).
type cartLine struct{ item, room, n int }

func (l cartLine) cost() Cost {
	c := shop[l.item].Cost
	return Cost{c[Money] * Num(l.n), c[Ecto] * Num(l.n)}
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
	return g.earned[Money]-since[Money] >= it.Earned[Money] && g.earned[Ecto]-since[Ecto] >= it.Earned[Ecto]
}

// owned: an unlock already bought, on its way, or already in the cart.
func (g *Game) owned(i int) bool {
	it := shop[i]
	return it.Tab == tabHotel && (g.unlocked(it.ID) || g.coming(it.ID, -1) || slices.ContainsFunc(g.cart, func(l cartLine) bool { return l.item == i }))
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
		if g.coming(it.ID, room) {
			return "ON ITS WAY TO ROOM " + g.hotel.Rooms[room].Number
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
	return g.afford(Cost{t[Money] + c[Money], t[Ecto] + c[Ecto]})
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
		t[Ecto] += c[Ecto]
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

// ship sends a paid order on its way (D200): one courier from the Entrance takes the Supplies and unlocks
// to the stock, then leaves. Priests start at once: they walk in on
// their own.
func (g *Game) ship(order []cartLine) {
	c := &courier{Guest: entities.NewGuest(g.hotel.Spawn, courierName)}
	stock := drop{at: g.stockDrop()}
	for _, l := range order {
		switch it := shop[l.item]; {
		case it.ID == Mirror: // installed at its own wall spot, not at the stock
			c.drops = append(c.drops, drop{at: g.hotel.Mirror, lines: []cartLine{l}})
		case it.Tab != tabServices:
			stock.lines = append(stock.lines, l)
		default:
			g.receive(l)
		}
	}
	if len(stock.lines) > 0 {
		c.drops = append([]drop{stock}, c.drops...)
	}
	if len(c.drops) > 0 {
		c.SetPath(g.hotel.Path(g.hotel.Spawn, c.drops[0].at, -1))
		g.couriers = append(g.couriers, c)
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

// price: "$40", "80 ECTOPLASM" or both.
func price(c Cost) string {
	var p []string
	if c[Money] > 0 {
		p = append(p, "$"+fmtNum(c[Money]))
	}
	if c[Ecto] > 0 {
		p = append(p, fmtNum(c[Ecto])+" ECTOPLASM")
	}
	return strings.Join(p, " + ")
}
