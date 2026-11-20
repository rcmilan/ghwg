package game

import "ghosthotel/entities"

// Fathers K and M as guests (08 T379, G2): once his call is unlocked a Father may book as an ordinary solo human
// guest, about 1 booking in priestGuestOdds. He pays priestRentMul times the Rate; while he is in his Room its
// ghosts leave (no Ecto) and it books none, and each game hour he cleans its spiritual dirt. Placeholders.
const (
	priestGuestOdds = 20
	priestRentMul   = 3
)

var fatherLines = [2][]string{
	[]string{"Mind the corners. Dust gathers where the dead linger.", "A quiet room, please. Quieter than that.", "I bless the pillows. Habit."},
	[]string{"I will be quiet. The Room will not.", "Something here owes me an exorcism.", "Leave the door open. They should see me coming."}}

// fatherOf: which Father the guest is (strong is Father M), false for anyone else.
func fatherOf(guest *entities.Guest) (strong, ok bool) {
	return guest.Character == fatherM.name, guest.Character == fatherK.name || guest.Character == fatherM.name
}

// guestFather: the Party is a Priest guest, and which Father.
func guestFather(p *Party) (strong, ok bool) {
	if p.Realm != Human || len(p.Guests) != 1 {
		return false, false
	}
	return fatherOf(p.Guests[0])
}

// fatherGuest: that Father is in the hotel as a guest.
func (g *Game) fatherGuest(strong bool) bool {
	for _, p := range g.parties {
		if s, ok := guestFather(p); ok && s == strong {
			return true
		}
	}
	return false
}

// maybePriestGuest turns a fresh solo booking into a Father's, when one is unlocked and not already in the hotel
// (fatherIn counts a retained or called Father, and a guest). The roll hashes g.walkIns, drawing from no stream.
func (g *Game) maybePriestGuest(p *Party) {
	if len(p.Guests) != 1 || p.Guests[0].Who.Key != "" { // a named guest stays who it is
		return
	}
	var from []bool
	for _, u := range [2]Unlock{unlockK, unlockM} {
		if strong := u == unlockM; g.unlocked(u) && !g.fatherIn(strong) {
			from = append(from, strong)
		}
	}
	h := uint32(g.walkIns) * 2654435761 // a hash of the booking count: no random stream is drawn, so the sim's never shifts
	if len(from) == 0 || h>>8%priestGuestOdds != 0 {
		return
	}
	g.becomeFather(p, from[h>>4%uint32(len(from))])
}

func (g *Game) becomeFather(p *Party, strong bool) {
	guest := p.Guests[0]
	guest.Character = fatherName(strong)
	guest.Who = entities.Identity{Name: guest.Character, Age: 52 + 12*b2i(strong), Job: "priest", Lines: fatherLines[b2i(strong)]}
	p.Rate *= priestRentMul
}

// stays: a Priest guest is in the Room.
func (g *Game) stays(room int) bool {
	for _, p := range g.parties {
		if _, ok := guestFather(p); ok && p.Room == room && g.inRoom(p) {
			return true
		}
	}
	return false
}

// updatePriestGuests: each game minute, a staying Priest guest's Room sends its ghosts on, without a payout; each
// game hour he scrubs its spiritual dirt.
func (g *Game) updatePriestGuests() {
	if !g.ticked || g.clock%minuteFrames != 0 {
		return
	}
	for _, p := range g.parties {
		if _, ok := guestFather(p); !ok || !g.inRoom(p) {
			continue
		}
		for _, q := range g.parties {
			if q.Realm == Ghost && q.Room == p.Room && q.state == staying && q.Guests[0].PassAt == 0 {
				for _, ghost := range q.Guests {
					ghost.PassAt, ghost.Path = g.clock, nil
				}
			}
		}
		if g.clock%hourFrames == 0 {
			g.scrub(p.Room, true)
		}
	}
}
