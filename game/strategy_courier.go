package game

// restockAt: a Supply's stock under this gets a case ordered (placeholder).
const restockAt = 2

// restock orders a case of a Supply when its stock runs low, it can pay, and no courier is already on his
// way (T241); true if it did. server, smart and keeper restock: without it they never serve (E5).
func restock(v View, p *Pilot) bool {
	for _, i := range v.Shop {
		it := shop[i]
		if !v.Terminal.Open && !v.Courier && it.Tab == tabSupplies && v.Stock[it.ID] < restockAt && v.Money >= it.Cost[Money] {
			p.Buy(it.ID, -1, func(View) bool { return true })
			return true
		}
	}
	return false
}
