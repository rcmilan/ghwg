package game

func init() { Strategies["keeper"] = keeper{} }

// keeper serves in real and cleans empty Rooms' dirt before their next Party (T245): in whatever
// dimension it's in, so it never walks to the Mirror for the Spook.
type keeper struct{}

func (keeper) Act(v View, p *Pilot) {
	if v.Terminal.Open {
		p.Close()
		return
	}
	if i := toServe(v); i >= 0 && !v.Ghost {
		p.Serve(i)
		return
	}
	for i, r := range v.Rooms {
		if (r.Vacant || r.Away) && len(r.Dirt) > 0 {
			p.Clean(i, r.Dirt[0])
			return
		}
	}
	if v.Ghost {
		p.Flip(false)
	}
}
