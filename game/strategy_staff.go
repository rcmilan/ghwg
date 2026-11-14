package game

import "slices"

func init() { Strategies["staffer"] = staffer{} }

// staffer leaves the service to a housekeeper (07 H1 step 3): it opens Rooms, keeps the stock up and hires
// one as soon as it can pay the fee and a first wage, then stands at the Terminal. Its Runs say whether a
// hotel's ordinary takings keep a housekeeper paid.
type staffer struct{}

func (staffer) Act(v View, p *Pilot) {
	if openRooms(v, p) || restock(v, p) {
		return
	}
	i := slices.IndexFunc(shop, func(it Item) bool { return it.ID == rowKeeper })
	if !v.Terminal.Open && v.Staff == 0 && slices.Contains(v.Shop, i) && v.Money >= keeperFee+keeperWage {
		p.Buy(rowKeeper, -1, func(v View) bool { return v.Staff > 0 })
		return
	}
	if i := toServe(v); i >= 0 && v.Staff == 0 && !v.Terminal.Open { // until one is hired, it serves itself
		serveRoom(v, p, i)
		return
	}
	stand{atTerminal}.Act(v, p)
}
