package game

import "slices"

func init() { Strategies["opener"] = opener{} }

// openRooms buys the key of the first boarded Room while the wallet covers it and no courier is out (07 B1);
// true if it ordered. server, smart and keeper call it first: a Run starts with every Room closed.
func openRooms(v View, p *Pilot) bool {
	if v.Terminal.Open || v.Courier {
		return false
	}
	owned := 0
	for _, r := range v.Rooms {
		if !r.Boarded {
			owned++
		}
	}
	if v.Money < keyPrice(owned) {
		return false
	}
	for i, r := range v.Rooms {
		if r.Boarded {
			p.Buy(rowKey, i, func(v View) bool { return v.Courier })
			return true
		}
	}
	return false
}

// upgrade plays K1's part of the opening, in order: Twin beds once on sale, a Single Room changed to Twin, and
// the in-Room Mirror, bought for a Twin Room, as soon as the Ectoplasm is there. true if it ordered.
func upgrade(v View, p *Pilot) bool {
	row := func(id Unlock) int { return slices.IndexFunc(shop, func(it Item) bool { return it.ID == id }) }
	has := func(c Cost) bool { return v.Money >= c[Money] && v.Ecto >= c[Ecto] }
	if v.Terminal.Open || v.RoomMirror {
		return false
	}
	twin := slices.IndexFunc(v.Rooms, func(r RoomView) bool { return !r.Boarded && (r.Changing || r.Beds == "Twin") }) // changed, or about to
	if i := row(RoomMirror); twin >= 0 && !v.Courier && slices.Contains(v.Shop, i) && has(shop[i].Cost) {              // the courier brings it (T1)
		p.Buy(RoomMirror, twin, func(v View) bool { return v.Courier })
		return true
	}
	if i := row(BedsTwin); !v.Courier && slices.Contains(v.Shop, i) && has(shop[i].Cost) {
		p.Buy(BedsTwin, -1, func(v View) bool { return v.Courier })
		return true
	}
	if single := slices.IndexFunc(v.Rooms, func(r RoomView) bool { return !r.Boarded && r.Beds == "Single" }); twin < 0 && single >= 0 && slices.Contains(v.Beds, "Twin") && has(Cost{Money: bedKinds[1].Fee}) {
		p.Change(single, "Twin")
		return true
	}
	return false
}

// opener plays the opening: the first key, then the Mirror as soon as it can pay (the pacing goal: reach the
// SHIFT feature early), then the other keys; once the Mirror is in it plays smart, upgrading its Rooms on
// the way to the in-Room Mirror (upgrade). Until the Mirror it waits at the Terminal and serves nobody.
type opener struct{}

func (opener) Act(v View, p *Pilot) {
	if v.Terminal.Open {
		p.Close()
		return
	}
	anyOpen := slices.ContainsFunc(v.Rooms, func(r RoomView) bool { return !r.Boarded })
	if anyOpen && v.MirrorLocked && !v.Courier {
		if i := slices.IndexFunc(shop, func(it Item) bool { return it.ID == Mirror }); slices.Contains(v.Shop, i) && v.Money >= shop[i].Cost[Money] {
			p.Buy(Mirror, 0, func(v View) bool { return v.Courier })
			return
		}
	}
	if !v.MirrorLocked {
		if !upgrade(v, p) {
			saving := !slices.ContainsFunc(v.Rooms, func(r RoomView) bool { return !r.Boarded && (r.Changing || r.Beds == "Twin") })
			smart{save: saving}.Act(v, p) // it opens the other Rooms too
		}
		return
	}
	if !openRooms(v, p) {
		stand{atTerminal}.Act(v, p)
	}
}
