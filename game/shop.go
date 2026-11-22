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
	tabStaff               // the people to hire (keeper.go)
	tabUpgrades            // the leveled upgrades (upgrade_rows.go)
	tabCart
	tabs
	tabRooms // not a tab of the store: all that is bought for one Room, in the hotel view, paid at once (hotelview.go)
)

var tabNames = [tabs]string{"SUPPLIES", "HOTEL", "STAFF", "UPGRADES", "CART"}

// Item is one product of the store.
type Item struct {
	ID     Unlock
	Tab    Tab
	Name   string
	Blurb  string // one cartLine on the detail pane
	Hint   string // what unlocks it, shown while it's still a teaser (08 F0-T fills these)
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
	Hire   bool // staff: one more walks in each time it is bought (keeper.go)
	Shoes  int  // running shoes: the tier (1..3), bought in order; the row of the next one shows (keeper.go)
	Grow   int  // H1: growCol or growFloor: the hotel grows as the Terminal closes. Bought again and again, one at a time
	Up     int  // a leveled upgrade (upgrade_rows.go): its Upgrade + 1. One level an order, delivered to the stock
}

// room: the item is for one Room: an action of the hotel view.
func (it Item) room() bool { return it.Send != nil }

// unlock: the item is an unlock, owned once bought: a HOTEL item not for a Room, or a Once one.
func (it Item) unlock() bool {
	return it.Tab == tabHotel && !it.room() && it.Grow == 0 && it.Up == 0 && it.ID != CorridorMirror && it.ID != rowStorm && !isQuestRow(it.ID) || it.Once // the Corridor mirror is bought again and again; the Storm jar once a day (items.go); a quest row is an order (08 W5)
}

// The Services aren't unlocks; their IDs only name them.
const (
	rowFatherM Unlock = "father-m"
)

func noFatherM(g *Game, room int) string { return g.noFather(room, true) }

// shop is the catalog, in its order: a tab lists its rows as they stand here, so a new row goes next to its
// kind (the keys, then the Priests), not at the end. Prices, packs and thresholds are placeholders.
var shop = []Item{
	{ID: SupplyWater, Tab: tabSupplies, Name: "Water", Blurb: "Bottled at the source. Do not ask which.", Hint: "Buy a Room to unlock.", Icon: "icon_water", Cost: Cost{Money: 50}, Pack: 5},
	{ID: SupplyFood, Tab: tabSupplies, Name: "Food", Blurb: "Hot meals, mostly from this dimension.", Hint: "Buy a Room to unlock.", Icon: "food_4", Cost: Cost{Money: 100}, Pack: 5},
	{ID: SupplyTowel, Tab: tabSupplies, Name: "Towels", Blurb: "Fluffy. Stains come out. Most stains.", Hint: "Buy a Room to unlock.", Icon: "icon_towel", Cost: Cost{Money: 75}, Pack: 5},

	{ID: Mirror, Tab: tabHotel, Name: "The Mirror", Blurb: "Totally not haunted. Ignore the knocking.", Hint: "Buy a Room to unlock.", Icon: "icon_mirror", Cost: Cost{Money: mirrorPrice}},
	{ID: BedsTwin, Tab: tabHotel, Name: "Twin beds", Blurb: "Two beds. Twice the snoring.", Hint: "Earn $50 after the Mirror to unlock.", Icon: "icon_bed_twin", Cost: Cost{Money: twinPrice},
		Needs: []Unlock{Mirror}, Earned: Cost{Money: 50}},
	{ID: BedsDouble, Tab: tabHotel, Name: "Double bed", Blurb: "One bed. Whose side is whose is their problem.", Hint: "Earn $200 after Twin beds to unlock.", Icon: "icon_bed_double", Cost: Cost{Money: 2 * twinPrice},
		Needs: []Unlock{BedsTwin}, Earned: Cost{Money: 200}},
	// Housekeeping (Money 50 + Ecto 20, "Cleans empty Rooms. Asks no questions.") returns with auto-clean:
	// manual cleaning is free from the start (E6 close-out, 2026-11-08).
	{ID: RoomLock, Tab: tabHotel, Name: "Room lock", Blurb: "A plate by each door: who is in, at a glance.", Hint: "Earn 100 Ectoplasm after the Mirror to unlock.", Icon: "lock_red", Cost: Cost{Money: 300, Ecto: 40},
		Needs: []Unlock{Mirror}, Earned: Cost{Ecto: 100}},
	{ID: Deluxe, Tab: tabHotel, Name: "Deluxe rooms", Blurb: "Finer rooms. Guests pay the difference.", Hint: "Grow the hotel to unlock.", Icon: "icon_deluxe", Cost: Cost{Ecto: deluxeEcto},
		Needs: []Unlock{Mirror}},
	{ID: GrowCol, Tab: tabHotel, Grow: growCol, Name: "New column", Blurb: "One more Room a floor. Keys sold separately.", Hint: "Buy the Mirror to unlock.", Icon: "icon_grow_col",
		Cost: Cost{Ecto: growEcto}, Price: growCost, Needs: []Unlock{Mirror}},
	{ID: GrowFloor, Tab: tabHotel, Grow: growFloor, Name: "New floor", Blurb: "Built overnight. Do not ask by whom.", Hint: "Buy the Mirror to unlock.", Icon: "icon_grow_floor",
		Cost: Cost{Ecto: growEcto}, Price: growCost, Needs: []Unlock{Mirror}},
	{ID: CorridorMirror, Tab: tabHotel, Name: "Corridor mirror", Blurb: "A courier uncovers a Corridor's Mirror. Shift from there.", Hint: "Grow a floor to unlock.", Icon: "icon_mirror", Cost: Cost{Money: uncoverMoney, Ecto: uncoverEcto}, Price: uncoverCost,
		Needs: []Unlock{Mirror}},
	{ID: Pantry, Tab: tabHotel, Name: "Pantry", Blurb: "A shelf on each new Corridor, stocked from the kitchen.", Hint: levelHint(Pantry), Icon: "shelf_b", Cost: Cost{Money: pantryMoney, Ecto: pantryEcto}},
	{ID: Matchmaking, Tab: tabHotel, Name: "Matchmaking", Blurb: "Housekeepers talk guests into rooms they suit.", Hint: "Own the Mirror and hire a housekeeper to unlock.", Icon: keeperName, Head: true,
		Cost: Cost{Ecto: matchEcto}, Needs: []Unlock{Mirror}}, // and a housekeeper hired (onSale)

	{ID: rowKeeper, Tab: tabStaff, Hire: true, Name: "Housekeeper", Blurb: "Serves, cleans, roams. Paid by the day, or gone.", Hint: "Earn $" + fmtNum(keeperEarned) + " to unlock.", Icon: keeperName, Head: true,
		Cost: Cost{Money: keeperFee}, Earned: Cost{Money: keeperEarned}},

	{ID: shoeIDs[0], Tab: tabStaff, Shoes: 1, Name: "Running shoes I", Blurb: "Staff walk 15% faster. Laces not included.", Hint: "Hire a housekeeper to unlock.", Icon: keeperName, Head: true, Cost: Cost{Ecto: shoePrice[0]}},
	{ID: shoeIDs[1], Tab: tabStaff, Shoes: 2, Name: "Running shoes II", Blurb: "Staff walk another 15% faster.", Hint: "Hire a housekeeper to unlock.", Icon: keeperName, Head: true, Cost: Cost{Ecto: shoePrice[1]}},
	{ID: shoeIDs[2], Tab: tabStaff, Shoes: 3, Name: "Running shoes III", Blurb: "Staff walk another 15% faster. Rude to ghosts.", Hint: "Hire a housekeeper to unlock.", Icon: keeperName, Head: true, Cost: Cost{Ecto: shoePrice[2]}},
	{ID: unlockK, Tab: tabStaff, Once: true, Name: nameFatherK, Blurb: nameFatherK + " can be kept on, for his Ritual round the Rooms.", Hint: "Earn $250 after the Mirror to unlock.", Icon: fatherK.name, Head: true,
		Cost: Cost{Ecto: unlockKEcto}, Needs: []Unlock{Mirror}, Earned: Cost{Money: 250}},
	{ID: unlockM, Tab: tabStaff, Once: true, Name: "Call " + nameFatherM, Blurb: nameFatherM + " clears a Room, banishes its Shadow, sends its ghosts over.", Hint: "Earn $750 after the Mirror to unlock.", Icon: fatherM.name, Head: true,
		Cost: Cost{Ecto: unlockMEcto}, Needs: []Unlock{Mirror}, Earned: Cost{Money: 750}},
	{ID: rowRetain, Tab: tabStaff, Name: "Retain " + nameFatherK, Blurb: nameFatherK + " stays and cleans the ghostliest Room. Ectoplasm a day, or gone.", Hint: "Buy " + nameFatherK + " to unlock.", Icon: fatherK.name, Head: true,
		Cost: Cost{Ecto: retainerEcto}, Needs: []Unlock{unlockK}},
	{ID: rowFatherP, Tab: tabStaff, Name: nameFatherP, Blurb: "Takes the hotel, gives a bonus on all income. Begin again.", Hint: "Fill the biggest hotel and buy every Room key to unlock.", Icon: fatherP.name, Head: true,
		Cost: Cost{Ecto: prestigeEcto}}, // SPACE opens his card (prestige.go), no cart
	// 08 W5: the quest rows (verbs.go, items.go); questRowOn says when they are on sale.
	{ID: rowWreath, Tab: tabHotel, Name: "Wreath", Blurb: wreathRow, Icon: "icon_wreath", Cost: Cost{Money: 200}},
	{ID: rowCrate, Tab: tabHotel, Name: "A crate", Blurb: crateRow, Icon: "icon_crate", Cost: Cost{Money: 2000, Ecto: 500}},
	{ID: rowStorm, Tab: tabHotel, Name: "Storm in a jar", Blurb: "Tonight's thunderstorm: sightings pay double. Free, once a day.", Icon: "icon_storm"},

	{ID: rowKey, Tab: tabRooms, Name: "Room key", Blurb: "Opens a boarded-up Room. Bring a hammer.", Icon: "icon_key", Cost: Cost{Money: keyBase}, // its Price is set in owned.go's init
		Send: func(g *Game, l cartLine) { g.openRoom(l.room) }},
	{ID: rowChange, Tab: tabRooms, Name: "Change type", Blurb: "New beds for a Room, once it's empty.", Icon: "icon_door", Cost: Cost{Money: bedKinds[0].Fee}, Price: changeCost,
		Send: func(g *Game, l cartLine) {
			t, _ := world.TypeIn(g.types, g.hotel.Rooms[l.room].Type.Name, l.beds) // the Room keeps its tier
			g.changes = append(g.changes, &change{room: l.room, to: t, paid: l.cost()})
		}},
	{ID: rowDeluxe, Tab: tabRooms, Name: "Deluxe", Blurb: "This Room, finer. Once it's empty.", Icon: "icon_deluxe", Cost: Cost{Ecto: deluxeFee},
		Needs: []Unlock{Deluxe}, Send: func(g *Game, l cartLine) {}}, // its Send and Refuse are set in roomtype_change.go's init
	{ID: rowFatherM, Tab: tabRooms, Name: nameFatherM, Blurb: "Clears a Room, banishes its Shadow, sends its ghosts over.", Icon: fatherM.name, Head: true, Cost: Cost{Ecto: priestStrong},
		Needs: []Unlock{unlockM}, Refuse: noFatherM, Send: func(g *Game, l cartLine) { g.sendPriest(l.room, true) }},
	{ID: RoomMirror, Tab: tabRooms, Once: true, Name: "Room mirror", Blurb: "A Mirror per Room. A second ghost checks in through it.", Icon: "icon_mirror", Cost: Cost{Money: roomMirrorMoney, Ecto: roomMirrorEcto},
		Needs: []Unlock{Mirror}, Send: (*Game).buyRoomMirror},
	{ID: rowAddMirror, Tab: tabRooms, Name: "Add mirror", Blurb: "Hang a Mirror here. A second ghost checks in through it.", Icon: "icon_mirror", Cost: Cost{Money: addMirrorMoney, Ecto: addMirrorEcto},
		Needs: []Unlock{RoomMirror}, Refuse: noAddMirror, Send: (*Game).buyAddMirror},
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
	if it.Up > 0 { // on sale once it applies (upShows)
		return g.upShows(Upgrade(it.Up - 1))
	}
	if g.stepNeeds(it.ID) { // 08 W5 R9: a row the quest's step needs is on sale, whatever its gates
		return true
	}
	if it.ID == rowFatherP {
		return g.prestigeReady()
	}
	if on, quest := g.questRowOn(it.ID); quest { // 08 W5
		return on
	}
	if on, row := g.itemRowOn(it.ID); row { // 08 W5: Gerald, the Ward
		return on
	}
	if it.ID == CorridorMirror { // on sale while one is covered, however the Run got its New floor (08 B1 live test 3)
		return g.nextCovered() >= 0
	}
	if g.level < unlockLevel(it.ID) { // a prestige level opens it (pantry.go)
		return false
	}
	if g.ownedCount() == 0 && it.ID != rowKey { // no Room open yet: only a key is on sale, so no buy can strand the Run (2026-11-11)
		return false
	}
	if it.ID == Deluxe && !g.grown() || (it.ID == Matchmaking || it.Shoes != 0) && g.staffed() == 0 { // the hotel expands first (2026-11-14); someone to do the matching (X2)
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
	if it.Up > 0 { // at its top level, or a level in the cart: nothing more to add
		return g.upMaxed(Upgrade(it.Up-1)) || g.inCart(i) > 0
	}
	if it.Shoes != 0 {
		return slices.ContainsFunc(g.cart, func(l cartLine) bool { return l.item == i })
	}
	return it.unlock() && (g.unlocked(it.ID) || g.coming(it.ID, -1) || slices.ContainsFunc(g.cart, func(l cartLine) bool { return l.item == i }))
}

// addToCart puts item i in the cart, for the Room of a Service: "" or why not.
func (g *Game) addToCart(i, room int) string {
	it := shop[i]
	switch {
	case !g.onSale(i):
		return "COMING SOON"
	case it.ID == rowFatherP: // his card, not the cart
		return "FATHER P ASKS FIRST"
	case it.unlock() && g.unlocked(it.ID):
		return "ALREADY OWNED"
	case it.Up > 0 && g.upMaxed(Upgrade(it.Up-1)):
		return "MAX LEVEL"
	case it.Up > 0 && g.coming(it.ID, -1): // its price is the next level's: one level at a time
		return "ON ITS WAY"
	case g.owned(i), (it.ID == CorridorMirror || isQuestRow(it.ID)) && g.inCart(i) > 0: // the next one's price fixes it: one at a time
		return "ALREADY IN THE CART"
	case it.ID == rowRetain && g.fatherIn(false):
		return "FATHER K IS ALREADY IN THE HOTEL"
	case it.ID == rowRetain && g.inCart(i) > 0:
		return "ALREADY IN THE CART"
	case !g.affords(i):
		return "CAN'T AFFORD"
	case it.Hire && g.staffed()+g.inCart(i) >= keeperCap(g.ownedCount()):
		return "FULLY STAFFED"
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
	l := cartLine{item: i, room: -1, n: 1, each: g.priceOf(i)}
	if it.ID == CorridorMirror {
		l.room = g.nextCovered() // the Mirror the courier uncovers (not remapped by a growth: not a Room action)
	}
	g.cart = append(g.cart, l)
	return ""
}

// inCart: units of item i in the cart.
func (g *Game) inCart(i int) int {
	n := 0
	for _, l := range g.cart {
		if l.item == i {
			n += l.n
		}
	}
	return n
}

// affords: the cart plus one more of item i fits the wallet (2026-11-08: no adding what you can't pay).
func (g *Game) affords(i int) bool {
	t, c := g.cartTotal(), g.priceOf(i)
	return g.afford(Cost{t[Money] + c[Money], t[Ecto] + c[Ecto]})
}

// sameKind: both Priests (one per Room), both changes to a Room (its type, its Mirror), or the same Service.
func sameKind(a, b int) bool {
	priest := func(i int) bool { return shop[i].ID == rowFatherM }
	change := func(i int) bool {
		return shop[i].ID == rowChange || shop[i].ID == RoomMirror || shop[i].ID == rowAddMirror
	}
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
		case it.ID == rowStorm: // no courier: the jar is shaken at once (items.go)
			g.shakeJar()
		case it.Hire: // no courier: they walk in themselves
			g.staffPaid += l.cost()[Money]
			for range l.n {
				g.hire()
			}
		case it.Shoes != 0: // no courier: the staff lace up at once
			g.own(it.ID)
		case it.ID == rowRetain: // no courier: he walks in himself, on his retainer
			g.retainK()
		case it.Grow != 0: // no courier: the hotel grows as the Terminal closes (grow.go)
			g.growNext = it.Grow
		case it.ID == Mirror: // installed at its own wall spot, not at the stock
			c.drops = append(c.drops, drop{at: g.hotel.Mirror, lines: []cartLine{l}})
		case it.ID == CorridorMirror: // uncovered where it hangs, from the Corridor cell below it
			c.drops = append(c.drops, drop{at: g.spotDrop(l.room), lines: []cartLine{l}})
		case it.ID == RoomMirror || it.ID == rowAddMirror: // walked into the Room, to where the Mirror hangs
			c.drops = append(c.drops, g.intoRoom(l.room, g.hotel.RoomMirrorSpot(l.room), l)...)
		case it.ID == rowKey: // walked to the Room's own door
			c.drops = append(c.drops, drop{at: g.doorDrop(l.room), lines: []cartLine{l}})
		case isQuestRow(it.ID): // 08 W5: the wreath to Madame's door, the crate into Madame's Room
			c.drops = append(c.drops, g.shipQuest(l)...)
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
		c.pathTo(g, c.drops[0])
		g.couriers = append(g.couriers, c)
	}
}

// receive: an order cartLine arrives. A Supply goes into stock, an unlock is owned, a Service starts.
func (g *Game) receive(l cartLine) {
	it := shop[l.item]
	switch {
	case it.room():
		it.Send(g, l)
	case isQuestRow(it.ID): // 08 W5
		g.receiveQuest(l)
	case it.Tab == tabSupplies:
		if g.stock == nil {
			g.stock = map[Unlock]int{}
		}
		g.stock[it.ID] += it.Pack * l.n
	case it.ID == CorridorMirror:
		g.hotel.Corridors[l.room].Open = true
	case it.ID == Pantry:
		g.boughtPantry()
	case it.Up > 0:
		g.levelUp(Upgrade(it.Up - 1))
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
