package game

// spendUp (D102): the Strategies buy the cheapest upgrade level on sale that Money covers after a reserve: a day's
// wages, the cheapest Money unlock on sale and, while a Room is boarded, its key (what builds the hotel comes first). true if it ordered.
func spendUp(v View, p *Pilot) bool {
	if NoUpgrades || v.Terminal.Open || v.Courier {
		return false
	}
	reserve, owned := keeperWage*Num(v.Staff+1), 0
	for _, r := range v.Rooms {
		if !r.Boarded {
			owned++
		}
	}
	if owned < len(v.Rooms) {
		reserve += keyPrice(owned)
	}
	next := Num(0) // the cheapest Money unlock or hire on sale: it is what the wallet is saving for
	for _, i := range v.Shop {
		if c := shop[i].Cost[Money]; shop[i].Tab != tabSupplies && shop[i].ID != rowKey && !shop[i].room() && c > 0 && (next == 0 || c < next) {
			next = c
		}
	}
	reserve += next
	best := Upgrade(-1)
	for u, x := range v.Ups {
		if x.Shown && !x.Max && v.Money >= x.Price+reserve && v.Ecto >= x.Ecto && (best < 0 || x.Price < v.Ups[best].Price) {
			best = Upgrade(u)
		}
	}
	if best < 0 {
		return false
	}
	p.BuyUp(best, v.Ups[best].Level)
	return true
}

// BuyUp orders the next level of u through the store like any item (Pilot.Buy): ok once it is paid for; the
// courier brings it (spendUp waits while he is in the hotel).
func (p *Pilot) BuyUp(u Upgrade, from int) {
	p.Buy(shop[upRow[u]].ID, -1, func(v View) bool { return v.Ups[u].Level > from || v.Courier })
}

// upSpent: the Money the upgrade levels bought since up cost (the sim's ledger).
func upSpent(g *Game, up [upgradeCount]int) (n int) {
	for u := range upgradeCount {
		for l := up[u]; l < g.up[u]; l++ {
			n += int(upPriceAt(u, l, g.ownedCount()))
		}
	}
	return
}

// upTotals: the levels bought this Run and what they cost in all (at the Rooms owned now: a sim figure).
func upTotals(g *Game) (levels, cost int) {
	var none [upgradeCount]int
	for _, l := range g.up {
		levels += l
	}
	return levels, upSpent(g, none)
}

// NoUpgrades keeps the Strategies from buying upgrade levels (the sim's -noup, for a before/after).
var NoUpgrades bool
