package game

import (
	"slices"
	"strings"
)

// Item is a row of the Terminal's shop (D180).
type Item struct {
	ID      Unlock
	Name    string
	Head    string // a character's head drawn on the row; "" none yet
	Cost    Cost
	Needs   []Unlock
	PerRoom bool                           // bought for the Room picked with A/D
	Buy     func(g *Game, room int) string // pays its own price: "" or why not. nil: a one-time unlock of ID, paid by buy
}

// The rows that aren't unlocks: bought again and again, never leaving the list.
const (
	rowFatherK Unlock = "father-k"
	rowFatherM Unlock = "father-m"
	rowShadow  Unlock = "shadow"
)

// shop is everything the Terminal sells, in its order. Unlock costs are placeholders (T183).
var shop = []Item{
	{ID: rowFatherK, Name: "Father K: halves Spook", Head: fatherK.name, Cost: Cost{Money: priestWeak}, Needs: []Unlock{Mirror}, PerRoom: true,
		Buy: func(g *Game, room int) string { return g.callPriest(room, false) }},
	{ID: rowFatherM, Name: "Father M: clears it, banishes a Shadow", Head: fatherM.name, Cost: Cost{Money: priestStrong}, Needs: []Unlock{Mirror}, PerRoom: true,
		Buy: func(g *Game, room int) string { return g.callPriest(room, true) }},
	{ID: rowShadow, Name: "Shadow Person", Head: "Adam", Cost: Cost{Spook: shadowPrice}, Needs: []Unlock{Mirror}, PerRoom: true,
		Buy: func(g *Game, room int) string { return g.buyShadow(room) }},
	{ID: Terminal, Name: "Terminal", Cost: Cost{Money: 5}},
	{ID: Mirror, Name: "The Mirror", Cost: Cost{Money: 10}},
	{ID: SupplyWater, Name: "Water", Cost: Cost{Money: 15}},
	{ID: Hex, Name: "Hex", Cost: Cost{Money: 25}, Needs: []Unlock{Mirror}},
	{ID: SupplyFood, Name: "Food", Cost: Cost{Money: 40}, Needs: []Unlock{SupplyWater}},
	{ID: Housekeep, Name: "Housekeeping", Cost: Cost{Money: 50, Spook: 20}, Needs: []Unlock{Mirror}},
	{ID: RoomLock, Name: "Room lock", Cost: Cost{Money: 60, Spook: 40}, Needs: []Unlock{Hex}},
	{ID: SupplyTowel, Name: "Towels", Cost: Cost{Money: 80, Spook: 10}, Needs: []Unlock{SupplyWater}},
	{ID: Consume, Name: "Consume", Cost: Cost{Money: 100, Spook: 50}, Needs: []Unlock{SupplyFood}},
	{ID: ShiftAnywhere, Name: "Shift anywhere", Cost: Cost{Money: 500, Spook: 500}, Needs: []Unlock{Mirror}},
}

// buy sells row i, for the Room for a PerRoom row: "" or why not.
func (g *Game) buy(i, room int) string {
	it := shop[i]
	if !g.afford(it.Cost) {
		return "NOT ENOUGH: " + price(it.Cost)
	}
	if it.Buy != nil {
		return it.Buy(g, room)
	}
	g.pay(it.Cost)
	g.unlock(it.ID)
	return ""
}

// reveal marks every row whose Needs are unlocked and whose Cost the wallet half holds, in each part.
// Once seen, a row stays listed (D180).
func (g *Game) reveal() {
	for _, it := range shop {
		if g.seen[it.ID] || slices.ContainsFunc(it.Needs, func(u Unlock) bool { return !g.unlocked(u) }) ||
			2*g.wallet[Money] < it.Cost[Money] || 2*g.wallet[Spook] < it.Cost[Spook] {
			continue
		}
		if g.seen == nil {
			g.seen = map[Unlock]bool{}
		}
		g.seen[it.ID] = true
	}
}

// listed: the rows the Terminal shows, by index: seen, less the unlocks bought.
func (g *Game) listed() []int {
	var rows []int
	for i, it := range shop {
		if g.seen[it.ID] && (it.Buy != nil || !g.unlocked(it.ID)) {
			rows = append(rows, i)
		}
	}
	return rows
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
