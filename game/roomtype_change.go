package game

import (
	"image"
	"math/rand/v2"
	"slices"
	"strings"

	"ghosthotel/entities"
	"ghosthotel/world"
)

// Room Types as options (07 K1). Every Room opens a Standard Single. Twin beds, then the Double bed, are
// HOTEL unlocks; the "Change type" Service switches a Room to any unlocked layout, Single too, each change
// for a fee. An empty Room changes at once, its reel rolling in the world when the Terminal closes; a Room
// in use waits, taking no booking in either realm, and changes when it empties. The in-Room Mirror is bought
// once, with Ectoplasm, for its first Room, where it is hung at once, guests in or not (the furniture doesn't
// change); after it every Room that changes, or that a key opens, has one,
// and every change costs Ectoplasm too. A Room's Mood stays with the Room. Amounts are placeholders, kept
// low for the pacing goal: the in-Room Mirror bought within 10 minutes of regular play (TestPacing).
const (
	BedsTwin   Unlock = "beds-twin"
	BedsDouble Unlock = "beds-double"
	Deluxe     Unlock = "deluxe" // the Deluxe tier (2026-11-14): bought with Ectoplasm, it puts the "Deluxe" upgrade on every Room
	rowDeluxe  Unlock = "upgrade-deluxe"
	RoomMirror Unlock = "room-mirror" // the in-Room Mirror, a Service bought once
	rowChange  Unlock = "change-type"

	reelFrames     = 60  // one Room's reel roll, ~1 s (D48)
	roomMirrorEcto = 120 // Ectoplasm: the in-Room Mirror
	changeEcto     = 5   // Ectoplasm: each change, once the in-Room Mirror is bought
	deluxeEcto     = 120 // Ectoplasm: the Deluxe tier
	deluxeFee      = 40  // Ectoplasm: a Room's upgrade to Deluxe
	deluxeTier     = "Deluxe"
)

// bedKinds: the layouts a Room changes between, the unlock each needs ("" for none) and its fee. A change keeps
// the Room's tier: a Deluxe Room stays Deluxe.
type bedKind struct {
	Beds  string
	Needs Unlock
	Fee   Num
}

var bedKinds = [...]bedKind{{"Single", "", 50}, {"Twin", BedsTwin, 75}, {"Double", BedsDouble, 125}}

// typeLabel: a Room Type as the hotel view names it: "TWIN", "DELUXE TWIN".
func typeLabel(t world.RoomType) string {
	if t.Name == deluxeTier {
		return "DELUXE " + strings.ToUpper(t.Beds)
	}
	return strings.ToUpper(t.Beds)
}

// noDeluxe: why the Room can't be upgraded to Deluxe, or "".
func noDeluxe(g *Game, room int) string {
	r := g.hotel.Rooms[room]
	switch {
	case !g.isOwned(room):
		return "ROOM " + r.Number + " IS BOARDED UP"
	case g.changing(room):
		return "ROOM " + r.Number + " IS ALREADY CHANGING"
	case r.Type.Name == deluxeTier:
		return "ROOM " + r.Number + " IS ALREADY DELUXE"
	}
	return ""
}

// upgrade: the Room's paid-for upgrade to Deluxe (2026-11-14): a change to the Deluxe of the beds it has, so it
// waits for the Room to empty and reels like one. The Room pays the Deluxe Rate from its next Booking on.
func (g *Game) upgrade(l cartLine) {
	t, _ := world.TypeIn(g.types, deluxeTier, g.hotel.Rooms[l.room].Type.Beds)
	g.changes = append(g.changes, &change{room: l.room, to: t, paid: l.cost()})
}

// change is a Room's paid-for change of type: waiting for the Room to empty, or its reel rolling.
type change struct {
	room int
	to   world.RoomType
	paid Cost // refunded if it's cancelled while it waits
	reel int  // the tick its reel began: the Room has changed; 0 while it waits
	pics reelPics
}

// bedOpen: bed layout k is unlocked.
func (g *Game) bedOpen(k int) bool { return bedKinds[k].Needs == "" || g.unlocked(bedKinds[k].Needs) }

// pickBeds moves the Terminal's picked layout d steps among the unlocked ones.
func (g *Game) pickBeds(d int) {
	for range bedKinds {
		if g.termBeds = (g.termBeds + d + len(bedKinds)) % len(bedKinds); g.bedOpen(g.termBeds) {
			return
		}
	}
}

// bedFee: a change to layout k: its fee, plus Ectoplasm once the in-Room Mirror is in.
func (g *Game) bedFee(k int) Cost {
	c := Cost{Money: bedKinds[k].Fee}
	if g.unlocked(RoomMirror) {
		c[Ecto] = changeEcto
	}
	return c
}

// changeCost: a change to the Terminal's picked layout.
func changeCost(g *Game) Cost { return g.bedFee(g.termBeds) }

// changing: the Room has a change waiting or rolling: it takes no booking.
func (g *Game) changing(room int) bool {
	return slices.ContainsFunc(g.changes, func(c *change) bool { return c.room == room })
}

// pending: the changes still waiting for their Rooms.
func (g *Game) pending() []*change {
	var w []*change
	for _, c := range g.changes {
		if c.reel == 0 {
			w = append(w, c)
		}
	}
	return w
}

func noChange(g *Game, room int) string {
	r := g.hotel.Rooms[room]
	switch {
	case !g.isOwned(room):
		return "ROOM " + r.Number + " IS BOARDED UP"
	case g.changing(room):
		return "ROOM " + r.Number + " IS ALREADY CHANGING"
	case !g.bedOpen(g.termBeds):
		return strings.ToUpper(bedKinds[g.termBeds].Beds) + " IS NOT UNLOCKED"
	case r.Type.Beds == bedKinds[g.termBeds].Beds && (!r.Mirror.Empty() || !g.unlocked(RoomMirror)): // the same type again only to get its Mirror
		return "ROOM " + r.Number + " IS ALREADY " + strings.ToUpper(r.Type.Beds)
	}
	return ""
}

func noRoomMirror(g *Game, room int) string {
	if !g.isOwned(room) {
		return "ROOM " + g.hotel.Rooms[room].Number + " IS BOARDED UP"
	}
	for r := range g.hotel.Rooms {
		if g.coming(RoomMirror, r) {
			return "THE ROOM MIRROR IS ON ITS WAY"
		}
	}
	return ""
}

// buyRoomMirror: the courier reached the Room's door with the in-Room Mirror (T1, 2026-11-13): it is owned, and
// its first Room gets it now, as it stands, guests in or not: no reel, no wait. A change the Room waits for
// keeps waiting.
func (g *Game) buyRoomMirror(l cartLine) {
	g.own(RoomMirror)
	g.setRoom(l.room, g.hotel.Rooms[l.room].Type, true)
}

// init sets the Refuse of K1's store rows here, as owned.go does: they read the shop.
func init() {
	for i := range shop {
		switch shop[i].ID {
		case rowChange:
			shop[i].Refuse = noChange
		case RoomMirror:
			shop[i].Refuse = noRoomMirror
		case rowDeluxe:
			shop[i].Refuse, shop[i].Send = noDeluxe, (*Game).upgrade
		}
	}
}

// cancelChange calls off a waiting change and refunds it in full; the Room books again.
func (g *Game) cancelChange(c *change) {
	g.wallet[Money] += c.paid[Money]
	g.wallet[Ecto] += c.paid[Ecto]
	g.changes = slices.DeleteFunc(g.changes, func(o *change) bool { return o == c })
}

// roomClear: nobody is in the Room nor has it booked, in either realm: no Party (a Tourist out for the day
// keeps it), no guest still walking out, not the Concierge, not a Priest.
func (g *Game) roomClear(room int) bool {
	shell := g.hotel.Rooms[room].Shell()
	in := func(x, y float64) bool { return image.Pt(int(x), int(y)).In(shell) }
	return g.vacant(room, Human) && g.vacant(room, Ghost) && !in(g.player.X, g.player.Y) &&
		!slices.ContainsFunc(g.priests, func(pr *priest) bool { return in(pr.X, pr.Y) }) &&
		!slices.ContainsFunc(g.guests(), func(guest *entities.Guest) bool { return in(guest.X, guest.Y) })
}

// updateChanges runs each frame of the world: a waiting change lands once its Room is clear, its reel rolling
// from then, seen or not; when the reel stops the Room books again.
func (g *Game) updateChanges() {
	kept := g.changes[:0]
	for _, c := range g.changes {
		switch {
		case c.reel == 0 && !g.reeling(c.room) && g.roomClear(c.room):
			c.reel = g.tick
			if !g.headless {
				c.pics[0] = g.roomPics(c.room)
			}
			g.setRoom(c.room, c.to, true)
			if !g.headless {
				c.pics[1] = g.roomPics(c.room)
			}
		case c.reel > 0 && g.tick-c.reel >= reelFrames:
			c.pics.free()
			continue
		}
		kept = append(kept, c)
	}
	clear(g.changes[len(kept):])
	g.changes = kept
}

// setRoom lands a Room Type on the Room, with its Mirror if it had one or (mirror) the in-Room Mirror is
// bought. The Room's Mood is the Room's, not the furniture's: it stays, and so does its dirt, each spot its kind
// and size (08 F0: re-spotting it all turned a ghost's dirt normal); a spot the new furniture covers moves to a
// bare cell.
func (g *Game) setRoom(room int, t world.RoomType, mirror bool) {
	g.hotel.Land(room, t, !g.hotel.Rooms[room].Mirror.Empty() || mirror && g.unlocked(RoomMirror))
	g.floorImg, g.stains = nil, nil // re-rendered on the next Draw
	if room >= len(g.dirt) {
		return
	}
	free, moved := g.floorCells(room), []int(nil)
	for k, s := range g.dirt[room] {
		if i := slices.Index(free, s.cell); i >= 0 {
			free = slices.Delete(free, i, i+1)
		} else {
			moved = append(moved, k)
		}
	}
	r := rand.New(rand.NewPCG(uint64(room)+1, uint64(g.dirtN[room])))
	for _, k := range moved {
		if len(free) > 0 {
			i := r.IntN(len(free))
			g.dirt[room][k].cell = free[i]
			free = slices.Delete(free, i, i+1)
		}
	}
}

// atRoomMirror: the Concierge stands under a Room's own Mirror, where SPACE shifts too.
func (g *Game) atRoomMirror() bool {
	return slices.ContainsFunc(g.hotel.Rooms, func(r world.Room) bool { return g.feet().In(r.Mirror) })
}

// DevChange is main's -change: a Run with everything unlocked and the Rooms open, the first Room's change to
// the bed layout ordered (it reels at once, mirrored) and the second's waiting on a Party that just moved
// in; the Concierge stands in the Corridor between them, both in view.
func (g *Game) DevChange(beds string) {
	k := slices.IndexFunc(bedKinds[:], func(b bedKind) bool { return strings.EqualFold(b.Beds, beds) })
	if k < 0 {
		return
	}
	g.locked, g.roomOwned, g.termBeds = nil, nil, k
	g.wallet = [2]Num{500, 500}
	if p := g.arrive(1); p != nil {
		g.settleIn(p)
	}
	i := slices.IndexFunc(shop, func(it Item) bool { return it.ID == rowChange })
	for room := range min(2, len(g.hotel.Rooms)) {
		g.addToCart(i, room)
	}
	g.placeOrder()
	o := g.hotel.Rooms[0].Origin // in the Corridor between the two Rooms, level with their middles
	g.player.X, g.player.Y = float64((o.X+world.RoomW+1)*world.TileSize), float64((o.Y+world.RoomH/2+1)*world.TileSize)
	w, h := g.mapSize()
	g.cam.Snap(g.player.X, g.player.Y, w, h)
}

// DevTerm is main's -term: the Terminal open on a tab and a row, SPACE pressed on it if place.
func (g *Game) DevTerm(tab, row int, place bool) {
	g.openTerminal()
	g.termTab = Tab(tab)
	if g.termPick = row; place {
		g.refuse(g.press())
	}
}
