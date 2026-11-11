package game

import (
	"image"
	"slices"
	"strings"

	"ghosthotel/entities"
	"ghosthotel/world"
)

// Room Types as options (07 K1). Every Room opens a Standard Single. Twin beds, then the Double bed, are
// HOTEL unlocks; the "Change type" Service switches a Room to any unlocked layout, Single too, each change
// for a fee. An empty Room changes at once, its reel rolling in the world when the Terminal closes; a Room
// in use waits, taking no booking in either realm, and changes when it empties. The in-Room Mirror is bought
// once, with Ectoplasm, for its first Room; after it every Room that changes, or that a key opens, has one,
// and every change costs Ectoplasm too. A Room's Mood stays with the Room. Amounts are placeholders, kept
// low: the pacing goal (the first in-Room Mirror near 10 minutes of regular play) is not met yet (TestPacing).
const (
	BedsTwin   Unlock = "beds-twin"
	BedsDouble Unlock = "beds-double"
	RoomMirror Unlock = "room-mirror" // the in-Room Mirror, a Service bought once
	rowChange  Unlock = "change-type"

	reelFrames     = 60 // one Room's reel roll, ~1 s (D48)
	roomMirrorEcto = 90 // Ectoplasm: the in-Room Mirror
	changeEcto     = 5  // Ectoplasm: each change, once the in-Room Mirror is bought
)

// bedKinds: the layouts a Room changes between, the unlock each needs ("" for none) and its fee.
type bedKind struct {
	Beds  string
	Needs Unlock
	Fee   Num
}

var bedKinds = [...]bedKind{{"Single", "", 10}, {"Twin", BedsTwin, 15}, {"Double", BedsDouble, 25}}

// change is a Room's paid-for change of type: waiting for the Room to empty, or its reel rolling.
type change struct {
	room  int
	to    world.RoomType
	paid  Cost // refunded if it's cancelled while it waits
	first bool // it carries the in-Room Mirror's purchase: cancelling it locks the Mirror again
	reel  int  // the tick its reel began: the Room has changed; 0 while it waits
	pics  reelPics
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

// changeCost: a change to the Terminal's picked layout: its fee, plus Ectoplasm once the in-Room Mirror is in.
func changeCost(g *Game) Cost {
	c := Cost{Money: bedKinds[g.termBeds].Fee}
	if g.unlocked(RoomMirror) {
		c[Ecto] = changeEcto
	}
	return c
}

// changing: the Room has a change waiting or rolling: it takes no booking.
func (g *Game) changing(room int) bool {
	return slices.ContainsFunc(g.changes, func(c *change) bool { return c.room == room })
}

// pending: the changes still waiting for their Rooms, the Terminal's CHANGES list.
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
	return ""
}

// buyRoomMirror: the in-Room Mirror is owned, and its first Room gets it: with the change that Room already
// waits for, if any (the two are then one: cancelled and refunded together), else with one of its own, to
// the type it has.
func (g *Game) buyRoomMirror(l cartLine) {
	g.own(RoomMirror)
	for _, c := range g.pending() {
		if c.room == l.room {
			c.first, c.paid[Ecto] = true, c.paid[Ecto]+l.cost()[Ecto]
			return
		}
	}
	g.changes = append(g.changes, &change{room: l.room, to: g.hotel.Rooms[l.room].Type, paid: l.cost(), first: true})
}

// init appends K1's store rows, and sets their Refuse here, as owned.go does: they read the shop.
func init() {
	shop = append(shop,
		Item{ID: BedsTwin, Tab: tabHotel, Name: "Twin beds", Blurb: "Two beds. Twice the snoring.", Icon: "drawer_a", Cost: Cost{Money: 20},
			Needs: []Unlock{Mirror}, Earned: Cost{Money: 10}},
		Item{ID: BedsDouble, Tab: tabHotel, Name: "Double bed", Blurb: "One bed. Whose side is whose is their problem.", Icon: "cabinet", Cost: Cost{Money: 40},
			Needs: []Unlock{BedsTwin}, Earned: Cost{Money: 40}},
		Item{ID: rowChange, Tab: tabServices, Name: "Change type", Blurb: "New beds for a Room, once it's empty.", Icon: "wardrobe_broken", Cost: Cost{Money: bedKinds[0].Fee}, Price: changeCost,
			Refuse: noChange, Send: func(g *Game, l cartLine) {
				t, _ := world.TypeOf(g.types, l.beds)
				g.changes = append(g.changes, &change{room: l.room, to: t, paid: l.cost()})
			}},
		Item{ID: RoomMirror, Tab: tabServices, Once: true, Name: "Room mirror", Blurb: "A Mirror of its own for every Room you change.", Icon: "cabinet_mirror", Cost: Cost{Ecto: roomMirrorEcto},
			Needs: []Unlock{Mirror}, Refuse: noRoomMirror, Send: (*Game).buyRoomMirror},
	)
}

// cancelChange calls off a waiting change and refunds it in full; the Room books again.
func (g *Game) cancelChange(c *change) {
	g.wallet[Money] += c.paid[Money]
	g.wallet[Ecto] += c.paid[Ecto]
	if c.first {
		if g.locked == nil {
			g.locked = map[Unlock]bool{}
		}
		g.locked[RoomMirror] = true
	}
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
// bought. The Room's Mood is the Room's, not the furniture's: it stays, and its dirt is spotted again on the
// new floor.
func (g *Game) setRoom(room int, t world.RoomType, mirror bool) {
	g.hotel.Land(room, t, !g.hotel.Rooms[room].Mirror.Empty() || mirror && g.unlocked(RoomMirror))
	g.floorImg, g.stains = nil, nil // re-rendered on the next Draw
	if room < len(g.dirt) {
		g.dirt[room] = nil
	}
}

// atRoomMirror: the Concierge stands under a Room's own Mirror, where SPACE shifts too.
func (g *Game) atRoomMirror() bool {
	return slices.ContainsFunc(g.hotel.Rooms, func(r world.Room) bool { return g.feet().In(r.Mirror) })
}

// DevChange is main's -change: a Run with everything unlocked and the Rooms open, the first Room's change to
// the bed layout ordered (it reels at once, mirrored) and the second's waiting on a Party that just moved
// in; the Concierge stands in the Corridor between them, both in view. With cart, the Terminal is open on its CART tab.
func (g *Game) DevChange(beds string, cart bool) {
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
	if cart {
		g.openTerminal()
		g.termTab, g.termPick = tabCart, 1
	}
}
