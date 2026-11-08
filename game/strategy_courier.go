package game

func init() { Strategies["restocker"] = restocker{} }

// restockAt: a Supply's stock under this gets a case ordered (placeholder).
const restockAt = 2

// restocker plays smart, and orders a case of a Supply when its stock runs low and no courier is already on
// his way (T241). Until E5 serves from the stock, it orders each Supply once.
type restocker struct{}

func (restocker) Act(v View, p *Pilot) {
	for _, i := range v.Shop {
		it := shop[i]
		if !v.Terminal.Open && !v.Courier && it.Tab == tabSupplies && v.Stock[it.ID] < restockAt && v.Money >= it.Cost[Money] {
			p.Buy(it.ID, -1, func(View) bool { return true })
			return
		}
	}
	smart{}.Act(v, p)
}
