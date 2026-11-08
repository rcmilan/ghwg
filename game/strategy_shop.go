package game

func init() { Strategies["shopper"] = shopper{} }

// shopper buys every Hotel unlock and Service on sale as soon as it can afford it (T186): an unlock once,
// a Priest for each Room without one, a Shadow for each Room without one. Supplies wait for E5 to use
// them. In between it plays smart.
type shopper struct{}

func (shopper) Act(v View, p *Pilot) {
	for _, i := range v.Shop {
		it := shop[i]
		if v.Terminal.Open || it.Tab == tabSupplies || v.Money < it.Cost[Money] || v.Spook < it.Cost[Spook] {
			continue
		}
		if it.Tab == tabHotel {
			id := it.ID
			p.Buy(id, 0, func(v View) bool { return true })
			return
		}
		for r, rv := range v.Rooms {
			switch {
			case it.ID == rowShadow && !rv.Shadow:
				p.BuyShadow(r)
				return
			case it.ID != rowShadow && !rv.Priest:
				p.CallPriest(r, it.ID == rowFatherM)
				return
			}
		}
	}
	smart{}.Act(v, p)
}
