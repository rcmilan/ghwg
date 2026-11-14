package game

import (
	"slices"
	"strings"

	"ghosthotel/entities"
	"ghosthotel/world"
)

// The Terminal is a web store (D180, redesigned 2026-11-08): tabs of items, a cart, a checkout.
type Tab int

const (
	tabSupplies Tab = iota // restock: into the hotel's stock
	tabHotel               // hotel-wide unlocks
	tabCart
	tabs
	tabRooms // not a tab of the store: all that is bought for one Room, in the hotel view, paid at once (hotelview.go)
)

var tabNames = [tabs]string{"SUPPLIES", "HOTEL", "CART"}

// Item is one product of the store.
type Item struct {
	ID     Unlock
	Tab    Tab
	Name   string
	Blurb  string // one cartLine on the detail pane
	Icon   string // assets/sprites/<Icon>.png, or a character whose head is the icon (Head)
	Head   bool
	Cost   Cost
	Price  func(g *Game) Cost // the price now, when it moves (the Room key); else Cost
	Pack   int                // Supplies: units into stock per buy
	Needs  []Unlock           // owned first, then
	Earned Cost               // earned since the last of Needs was bought. No Needs and no Earned: on sale from the start
	// For a Room only (Send set): why the Room can't take it ("" if it can), the paid-for delivery, and Once for one
	// bought a single time: an unlock that picks a Room (the in-Room Mirror).
	Refuse func(g *Game, room int) string
	Send   func(g *Game, l cartLine)
	Once   bool
	Grow   int // H1: growCol or growFloor: the hotel grows as the Terminal closes. Bought again and again, one at a time
}

// room: the item is for one Room: an action of the hotel view.
func (it Item) room() bool { return it.Send != nil }

// unlock: the item is an unlock, owned once bought: a HOTEL item not for a Room, or a Once one.
func (it Item) unlock() bool { return it.Tab == tabHotel && !it.room() && it.Grow == 0 || it.Once }

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

// shop is the catalog, in its order: a tab lists its rows as they stand here, so a new row goes next to its
// kind (the keys, then the Priests), not at the end. Prices, packs and thresholds are placeholders.
var shop = []Item{
	{ID: SupplyWater, Tab: tabSupplies, Name: "Water", Blurb: "Bottled at the source. Do not ask which.", Icon: "icon_water", Cost: Cost{Money: 50}, Pack: 5},
	{ID: SupplyFood, Tab: tabSupplies, Name: "Food", Blurb: "Hot meals, mostly from this dimension.", Icon: "food_4", Cost: Cost{Money: 100}, Pack: 5},
	{ID: SupplyTowel, Tab: tabSupplies, Name: "Towels", Blurb: "Fluffy. Stains come out. Most stains.", Icon: "icon_towel", Cost: Cost{Money: 75}, Pack: 5},

	{ID: Mirror, Tab: tabHotel, Name: "The Mirror", Blurb: "Totally not haunted. Ignore the knocking.", Icon: "icon_mirror", Cost: Cost{Money: mirrorPrice}},
	{ID: BedsTwin, Tab: tabHotel, Name: "Twin beds", Blurb: "Two beds. Twice the snoring.", Icon: "icon_bed_twin", Cost: Cost{Money: twinPrice},
		Needs: []Unlock{Mirror}, Earned: Cost{Money: 50}},
	{ID: BedsDouble, Tab: tabHotel, Name: "Double bed", Blurb: "One bed. Whose side is whose is their problem.", Icon: "icon_bed_double", Cost: Cost{Money: 2 * twinPrice},
		Needs: []Unlock{BedsTwin}, Earned: Cost{Money: 200}},
	// Housekeeping (Money 50 + Ecto 20, "Cleans empty Rooms. Asks no questions.") returns with auto-clean:
	// manual cleaning is free from the start (E6 close-out, 2026-11-08).
	{ID: RoomLock, Tab: tabHotel, Name: "Room lock", Blurb: "Checkout is whenever we say it is.", Icon: "lock_red", Cost: Cost{Money: 300, Ecto: 40},
		Needs: []Unlock{Mirror}, Earned: Cost{Ecto: 100}},
	{ID: Consume, Tab: tabHotel, Name: "Consume", Blurb: "Guests go in. Something else comes out.", Icon: "painting_scary_b", Cost: Cost{Money: 500, Ecto: 50},
		Needs: []Unlock{RoomLock}, Earned: Cost{Ecto: 200}},
	{ID: ShiftAnywhere, Tab: tabHotel, Name: "Shift anywhere", Blurb: "My husband shifted to another dimension. 5/5", Icon: "vanity", Cost: Cost{Money: 2500, Ecto: 500},
		Needs: []Unlock{Mirror}, Earned: Cost{Money: 5000, Ecto: 1000}},
	{ID: GrowCol, Tab: tabHotel, Grow: growCol, Name: "New column", Blurb: "One more Room a floor. Keys sold separately.", Icon: "icon_grow_col",
		Cost: Cost{Ecto: growEcto}, Price: growCost, Needs: []Unlock{Mirror}},
	{ID: GrowFloor, Tab: tabHotel, Grow: growFloor, Name: "New floor", Blurb: "Built overnight. Do not ask by whom.", Icon: "icon_grow_floor",
		Cost: Cost{Ecto: growEcto}, Price: growCost, Needs: []Unlock{Mirror}},

	{ID: rowKey, Tab: tabRooms, Name: "Room key", Blurb: "Opens a boarded-up Room. Bring a hammer.", Icon: "icon_key", Cost: Cost{Money: keyBase}, Price: keyCost,
		Send: func(g *Game, l cartLine) { g.openRoom(l.room) }},
	{ID: rowChange, Tab: tabRooms, Name: "Change type", Blurb: "New beds for a Room, once it's empty.", Icon: "icon_door", Cost: Cost{Money: bedKinds[0].Fee}, Price: changeCost,
		Send: func(g *Game, l cartLine) {
			t, _ := world.TypeOf(g.types, l.beds)
			g.changes = append(g.changes, &change{room: l.room, to: t, paid: l.cost()})
		}},
	{ID: rowFatherK, Tab: tabRooms, Name: "Father K", Blurb: "Halves a Room's dread. Paid by the hour.", Icon: fatherK.name, Head: true, Cost: Cost{Money: priestWeak},
		Needs: []Unlock{Mirror}, Earned: Cost{Money: 250}, Refuse: noPriest, Send: func(g *Game, l cartLine) { g.sendPriest(l.room, false) }},
	{ID: rowFatherM, Tab: tabRooms, Name: "Father M", Blurb: "Clears a Room, banishes a Shadow. Cash only.", Icon: fatherM.name, Head: true, Cost: Cost{Money: priestStrong},
		Needs: []Unlock{Mirror}, Earned: Cost{Money: 750}, Refuse: noPriest, Send: func(g *Game, l cartLine) { g.sendPriest(l.room, true) }},
	{ID: RoomMirror, Tab: tabRooms, Once: true, Name: "Room mirror", Blurb: "A Mirror of its own for every Room you change.", Icon: "icon_mirror", Cost: Cost{Ecto: roomMirrorEcto},
		Needs: []Unlock{Mirror}, Send: (*Game).buyRoomMirror},
}

// cartLine is one cart entry: n of a Supply, an unlock, or a Service for a Room (else room is -1).
type cartLine struct {
	item, room, n int
	each          Cost   // one unit's price, fixed when it went in the cart
	beds          string // a change of type: the bed layout picked (roomtype_change.go)
}

func (l cartLine) cost() Cost {
	return Cost{l.each[Money] * Num(l.n), l.each[Ecto] * Num(l.n)}
}

// onSale: its Needs are owned and, since the last of them was bought, the hotel earned its Earned.
// Buying never opens another item by itself (2026-11-08).
func (g *Game) onSale(i int) bool {
	it := shop[i]
	if g.ownedCount() == 0 && it.ID != rowKey { // no Room open yet: only a key is on sale, so no buy can strand the Run (2026-11-11)
		return false
	}
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
	return it.unlock() && (g.unlocked(it.ID) || g.coming(it.ID, -1) || slices.ContainsFunc(g.cart, func(l cartLine) bool { return l.item == i }))
}

// addToCart puts item i in the cart, for the Room of a Service: "" or why not.
func (g *Game) addToCart(i, room int) string {
	it := shop[i]
	switch {
	case !g.onSale(i):
		return "COMING SOON"
	case it.unlock() && g.unlocked(it.ID):
		return "ALREADY OWNED"
	case g.owned(i):
		return "ALREADY IN THE CART"
	case !g.affords(i):
		return "CAN'T AFFORD"
	case it.Grow != 0 && g.growRefuse(it.Grow) != "":
		return g.growRefuse(it.Grow)
	case it.room():
		if why := it.Refuse(g, room); why != "" {
			return why
		}
		if g.coming(it.ID, room) {
			return "ON ITS WAY TO ROOM " + g.hotel.Rooms[room].Number
		}
		if slices.ContainsFunc(g.cart, func(l cartLine) bool { return l.room == room && sameKind(l.item, i) }) {
			return "ALREADY IN THE CART FOR ROOM " + g.hotel.Rooms[room].Number
		}
		l := cartLine{item: i, room: room, n: 1, each: g.priceOf(i)}
		if it.ID == rowChange {
			l.beds = bedKinds[g.termBeds].Beds
		}
		g.cart = append(g.cart, l)
		return ""
	}
	if k := slices.IndexFunc(g.cart, func(l cartLine) bool { return l.item == i }); k >= 0 {
		g.cart[k].n++
		return ""
	}
	g.cart = append(g.cart, cartLine{item: i, room: -1, n: 1, each: g.priceOf(i)})
	return ""
}

// affords: the cart plus one more of item i fits the wallet (2026-11-08: no adding what you can't pay).
func (g *Game) affords(i int) bool {
	t, c := g.cartTotal(), g.priceOf(i)
	return g.afford(Cost{t[Money] + c[Money], t[Ecto] + c[Ecto]})
}

// sameKind: both Priests (one per Room), both changes to a Room (its type, its Mirror), or the same Service.
func sameKind(a, b int) bool {
	priest := func(i int) bool { return shop[i].ID == rowFatherK || shop[i].ID == rowFatherM }
	change := func(i int) bool { return shop[i].ID == rowChange || shop[i].ID == RoomMirror }
	return a == b || priest(a) && priest(b) || change(a) && change(b)
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
	var stops []drop // one per Supply ordered, at its prop (F1)
	stock := drop{at: g.stockDrop()}
	for _, l := range order {
		switch it := shop[l.item]; {
		case supplyOf(it.ID) >= 0:
			stops = append(stops, drop{at: g.supplyDrop(supplyOf(it.ID)), lines: []cartLine{l}})
		case it.Grow != 0: // no courier: the hotel grows as the Terminal closes (grow.go)
			g.growNext = it.Grow
		case it.ID == Mirror: // installed at its own wall spot, not at the stock
			c.drops = append(c.drops, drop{at: g.hotel.Mirror, lines: []cartLine{l}})
		case it.ID == rowKey || it.ID == RoomMirror: // walked to the Room's own door
			c.drops = append(c.drops, drop{at: g.doorDrop(l.room), lines: []cartLine{l}})
		case !it.room():
			stock.lines = append(stock.lines, l)
		default:
			g.receive(l)
		}
	}
	if len(stock.lines) > 0 {
		stops = append(stops, stock)
	}
	c.drops = append(stops, c.drops...)
	if len(c.drops) > 0 {
		c.SetPath(g.hotel.Path(g.hotel.Spawn, c.drops[0].at, -1))
		g.couriers = append(g.couriers, c)
	}
}

// receive: an order cartLine arrives. A Supply goes into stock, an unlock is owned, a Service starts.
func (g *Game) receive(l cartLine) {
	it := shop[l.item]
	switch {
	case it.room():
		it.Send(g, l)
	case it.Tab == tabSupplies:
		if g.stock == nil {
			g.stock = map[Unlock]int{}
		}
		g.stock[it.ID] += it.Pack * l.n
	default:
		g.own(it.ID)
	}
}

// own: an unlock arrives: it is owned from now, and what opens after it counts earnings from here.
func (g *Game) own(id Unlock) {
	g.unlock(id)
	if g.boughtAt == nil {
		g.boughtAt = map[Unlock][2]Num{}
	}
	g.boughtAt[id] = g.earned
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
