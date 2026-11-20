package game

import "slices"

// restockAt: a Supply's stock under this gets a case ordered (placeholder).
const restockAt = 2

// restock orders a case of a Supply when its stock runs low, it can pay, and no courier is already on his
// way (T241); true if it did. server, smart and keeper restock: without it they never serve (E5).
func restock(v View, p *Pilot) bool {
	for _, i := range v.Shop {
		it := shop[i]
		if !v.Terminal.Open && !v.Courier && it.Tab == tabSupplies && v.Stock[it.ID] < restockAt && v.Money >= it.Cost[Money] && (supplyOf(it.ID) < baseSupplies || asked(v, it.ID)) {
			p.Buy(it.ID, -1, func(View) bool { return true })
			return true
		}
	}
	return false
}

// asked: a Room's balloon shows item; a meal is restocked only once someone has asked for it (T378), so a case's price
// doesn't land the moment it goes on sale.
func asked(v View, item Unlock) bool {
	return slices.ContainsFunc(v.Rooms, func(r RoomView) bool { return r.Asking && r.Wants == item })
}
