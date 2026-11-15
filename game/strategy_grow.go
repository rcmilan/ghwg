package game

import "slices"

func init() { Strategies["grower"] = grower{} }

// grower plays the opener and expands the hotel whenever its Ectoplasm covers it (07 H1): a column, then a
// floor, turn about, up to the grid's limit. The new Rooms' keys are the opener's to buy.
type grower struct{}

func (grower) Act(v View, p *Pilot) {
	if c, r := v.Hotel.Grid(); !v.Terminal.Open && !v.Courier && !v.MirrorLocked {
		ids := []Unlock{GrowCol, GrowFloor}
		if c > r+1 || c >= maxCols {
			ids = []Unlock{GrowFloor, GrowCol}
		}
		for _, id := range ids {
			i := slices.IndexFunc(shop, func(it Item) bool { return it.ID == id })
			wide := id == GrowCol && c < maxCols || id == GrowFloor && r < maxRows
			if n := len(v.Rooms); wide && slices.Contains(v.Shop, i) && v.Ecto >= growPrice(c, r) {
				p.Buy(id, 0, func(v View) bool { return len(v.Rooms) > n })
				return
			}
		}
	}
	opener{}.Act(v, p)
}
