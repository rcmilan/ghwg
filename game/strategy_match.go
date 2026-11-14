package game

import "slices"

func init() {
	Strategies["matcher"] = matcher{onTheHouse}
	Strategies["asker"] = matcher{plainAsk} // the dumb rule it is measured against: always the free ask
}

// matcher plays goals and talks every bad pair it sees into a Room its human Party fits (07 X2), with one offer: the
// night on the house (sure, the Pilot picks the Room where it costs least), or the asker's free plain ask, again and
// again. It talks only to a Party in its Room, with somewhere to go and, on the house, the price in the wallet. It
// buys Matchmaking as soon as it can and leaves the bad pairs to the housekeepers from then on.
type matcher struct{ offer int }

func (m matcher) Act(v View, p *Pilot) {
	if !v.Terminal.Open && !v.Talk.Open && !v.Matchmaking {
		if i := slices.IndexFunc(shop, func(it Item) bool { return it.ID == Matchmaking }); slices.Contains(v.Shop, i) && !v.Courier && v.Ecto >= shop[i].Cost[Ecto] {
			p.Buy(Matchmaking, -1, func(v View) bool { return v.Courier })
			return
		}
		for i, r := range v.Rooms {
			if r.Bad && !r.Away && r.Fix > 0 && (m.offer == plainAsk || v.Money >= r.Fix) {
				p.Talk(i, m.offer)
				return
			}
		}
	}
	goals{}.Act(v, p)
}
