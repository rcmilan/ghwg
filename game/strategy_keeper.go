package game

func init() { Strategies["keeper"] = keeper{} }

// keeper keeps the supplies stocked, serves in real and cleans empty Rooms' dirt before their next Party (T245): in whatever
// dimension it's in, so it never walks to the Mirror for the Ecto.
type keeper struct{}

func (keeper) Act(v View, p *Pilot) {
	if v.Terminal.Open {
		p.Close()
		return
	}
	if openRooms(v, p) || restock(v, p) {
		return
	}
	if i := toServe(v); i >= 0 && !v.Ghost {
		serveRoom(v, p, i)
		return
	}
	for i, r := range v.Rooms {
		if !r.Boarded && len(r.Dirt) > 0 {
			p.Clean(i, r.Dirt[0])
			return
		}
	}
	if v.Ghost {
		p.Flip(false)
	}
}
