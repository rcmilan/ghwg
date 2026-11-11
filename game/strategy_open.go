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

// opener plays the opening: the first key, then the Mirror as soon as it can pay (the pacing goal: reach the
// SHIFT feature early), then the other keys; once the Mirror is in it plays smart. Until then it waits at the
// Terminal and serves nobody.
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
		smart{}.Act(v, p) // it opens the other Rooms too
		return
	}
	if !openRooms(v, p) {
		stand{atTerminal}.Act(v, p)
	}
}
