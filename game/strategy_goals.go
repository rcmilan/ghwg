package game

import "slices"

func init() { Strategies["goals"] = goals{} }

// goals plays a Run in the agreed order of first goals (2026-11-14, 4c): Room 1, the Mirror, Room 2, then all
// that earns, as soon as the wallet covers it. Ectoplasm goes, in order, on the first Growth, the in-Room
// Mirror, the Deluxe tier, a Room's upgrade to it, more Growth; Money on one housekeeper and the keys. In
// between it plays smart. The sim's pacing lines are read off its Runs: it buys whatever it can the moment it
// can, so when it stands idle there was nothing to buy and nothing to do.
type goals struct{}

func (goals) Act(v View, p *Pilot) {
	if v.Terminal.Open {
		p.Close()
		return
	}
	row := func(id Unlock) int { return slices.IndexFunc(shop, func(it Item) bool { return it.ID == id }) }
	sells := func(id Unlock) bool { return slices.Contains(v.Shop, row(id)) }
	buy := func(id Unlock, room int, c Cost, got func(View) bool) bool { // true if it ordered
		if v.Courier || !sells(id) || v.Money < c[Money] || v.Ecto < c[Ecto] {
			return false
		}
		p.Buy(id, room, got)
		return true
	}
	courier := func(v View) bool { return v.Courier }
	first := slices.IndexFunc(v.Rooms, func(r RoomView) bool { return !r.Boarded })
	switch {
	case first < 0: // smart buys the first key
	case v.MirrorLocked:
		if buy(Mirror, -1, shop[row(Mirror)].Cost, courier) {
			return
		}
	default:
		c, r := v.Hotel.Grid()
		n := len(v.Rooms)
		grow, grew := GrowCol, func(v View) bool { return len(v.Rooms) > n }
		if c >= maxCols {
			grow = GrowFloor
		}
		plain := slices.IndexFunc(v.Rooms, func(r RoomView) bool { return !r.Boarded && !r.Changing && r.Type != deluxeTier })
		for _, s := range []struct { // the first it wants is the one it saves for
			want bool
			id   Unlock
			room int
			cost Cost
			got  func(View) bool
		}{
			{growPrice(c, r) == growEcto, grow, -1, Cost{Ecto: growEcto}, grew},
			{!v.RoomMirror, RoomMirror, first, shop[row(RoomMirror)].Cost, courier},
			{sells(Deluxe), Deluxe, -1, shop[row(Deluxe)].Cost, courier},
			{plain >= 0 && sells(rowDeluxe), rowDeluxe, plain, shop[row(rowDeluxe)].Cost, func(v View) bool { return v.Rooms[plain].Changing || v.Rooms[plain].Type == deluxeTier }},
			{c < maxCols || r < maxRows, grow, -1, Cost{Ecto: growPrice(c, r)}, grew},
		} {
			if s.want {
				if buy(s.id, s.room, s.cost, s.got) {
					return
				}
				break
			}
		}
		if openRooms(v, p) { // the keys before the housekeeper
			return
		}
		if v.Staff == 0 && buy(rowKeeper, -1, Cost{Money: keeperFee + keeperWage}, func(v View) bool { return v.Staff > 0 }) {
			return
		}
	}
	smart{}.Act(v, p)
}
