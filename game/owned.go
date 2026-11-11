package game

import (
	"slices"

	"ghosthotel/assets"
	"ghosthotel/world"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
)

// Buy Rooms (07 B1): a new Run starts with every Room closed and unowned (boarded door, no bookings, shut to the
// Concierge) and Money for exactly one Room key. Keys are a Service of the Terminal's SERVICES tab, picking the
// Room; the courier walks the key to that Room's door and on the drop the Room is owned. The price of a key rises
// with each Room owned. All amounts are placeholders, chosen so a Run reaches the Mirror early (the user's
// pacing goal): the Mirror ($10, shop.go) is about 4 game hours of one Room's rent.
const (
	keyBase    = 20      // $: the first Room key
	keyStep    = 20      // $: each Room already owned adds this to the next key
	startMoney = keyBase // $: a Run starts with exactly one key's worth
)

// rowKey is the Room key's Service row.
const rowKey Unlock = "room-key"

// keyPrice: the key of the next Room, with n Rooms already owned.
func keyPrice(n int) Num { return Num(keyBase + keyStep*n) }

// isOwned: the Room is open for business. A nil g.roomOwned owns everything (the sim's Open, most tests).
func (g *Game) isOwned(room int) bool {
	return g.roomOwned == nil || room < 0 || room >= len(g.roomOwned) || g.roomOwned[room]
}

// ownedCount: Rooms owned.
func (g *Game) ownedCount() int {
	n := 0
	for i := range g.hotel.Rooms {
		if g.isOwned(i) {
			n++
		}
	}
	return n
}

// priceOf: what item i costs now: its own Price (the key's rises), else its Cost.
func (g *Game) priceOf(i int) Cost {
	if p := shop[i].Price; p != nil {
		return p(g)
	}
	return shop[i].Cost
}

func keyCost(g *Game) Cost { return Cost{Money: keyPrice(g.ownedCount())} }

// closeRooms is the opening state: no Room owned, Money for one key.
func (g *Game) closeRooms() {
	g.roomOwned = make([]bool, len(g.hotel.Rooms))
	g.wallet[Money] = startMoney
}

// openRoom: the key reached the Room's door. It is owned from now: its door unboards and its first Party and
// ghost book as at a Run's start.
func (g *Game) openRoom(room int) {
	if g.roomOwned == nil || g.isOwned(room) {
		return
	}
	g.roomOwned[room] = true
	g.book(room, firstGap)
	g.bookGhost(room, firstGap)
}

// keyPending: a key is in the cart or on its way. One at a time, so the price a line was added at is the
// price its drop leaves behind.
func (g *Game) keyPending() bool {
	return slices.ContainsFunc(g.cart, func(l cartLine) bool { return shop[l.item].ID == rowKey }) ||
		slices.ContainsFunc(g.couriers, func(c *courier) bool {
			return slices.ContainsFunc(c.drops, func(d drop) bool {
				return slices.ContainsFunc(d.lines, func(l cartLine) bool { return shop[l.item].ID == rowKey })
			})
		})
}

// init sets the key's Refuse here: noKey reads the shop, which would be an initialization cycle in its literal.
func init() {
	for i := range shop {
		if shop[i].ID == rowKey {
			shop[i].Refuse = noKey
		}
	}
}

func noKey(g *Game, room int) string {
	switch {
	case g.isOwned(room):
		return "ROOM " + g.hotel.Rooms[room].Number + " IS ALREADY OPEN"
	case g.keyPending():
		return "A KEY IS ALREADY ON ITS WAY"
	}
	return ""
}

// sprite: an assets/sprites image, loaded on first use.
func (g *Game) sprite(name string) *ebiten.Image {
	if g.icons == nil {
		g.icons = map[string]*ebiten.Image{}
	}
	img, ok := g.icons[name]
	if !ok {
		img, _, _ = ebitenutil.NewImageFromFileSystem(assets.FS, "sprites/"+name+".png")
		g.icons[name] = img
	}
	return img
}

// drawBoarded draws a closed Room's door as A3's boarded door, over the door's two cells (drawDoors).
func (g *Game) drawBoarded(screen *ebiten.Image, d world.Door, ox, oy int) {
	img := g.sprite("door_boarded")
	if img == nil {
		return
	}
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Scale(zoom, zoom)
	op.GeoM.Translate(float64(d.Rect.Min.X*zoom-ox), float64(d.Rect.Min.Y*zoom-oy))
	g.blit(screen, img, op, modeProp, 0, nil, g.roomShift(d.Room))
}

// DevKey is main's -key: the Run has already ordered the key of the Room (0-based), the Concierge standing in
// the Corridor by its door, to watch the courier bring it (use with -shot -at).
func (g *Game) DevKey(room int) {
	if room < 0 || room >= len(g.hotel.Rooms) {
		return
	}
	g.wallet[Money] = max(g.wallet[Money], keyPrice(g.ownedCount()))
	k := slices.IndexFunc(shop, func(it Item) bool { return it.ID == rowKey })
	if g.addToCart(k, room) == "" {
		g.placeOrder()
	}
	s := g.hotel.Rooms[room].Spot()
	g.player.X, g.player.Y = float64(s.Min.X-2*world.TileSize), float64(s.Max.Y)
	w, h := g.mapSize()
	g.cam.Snap(g.player.X, g.player.Y, w, h)
}
