package game

import (
	"strings"

	"ghosthotel/assets"

	"github.com/hajimehoshi/ebiten/v2"
)

// The ROOMS tab (07 T1): a grid of cards laid out like the hotel, following len(g.hotel.Rooms). Everything
// done to one Room is bought here and paid at once, no cart: SPACE on the Room, SPACE on the action; a change
// of type takes a third SPACE for its beds. g.termDepth is how far in: 0 the tab (A/D switches tabs), 1 the
// grid, 2 a Room's actions, 3 the beds of its change. ESC steps back one level; so does W from the grid's top row.

// roomCard is what a Room's card shows.
type roomCard struct {
	Title          string   // "101 SINGLE"
	Change         string   // the beds it is changing to, waiting or rolling; "" if none
	Boarded        bool     // not owned yet
	Mirror         bool     // it has its own Mirror
	Humans, Ghosts []string // its Parties' characters, a head each
	Shadow, Priest string   // its Shadow Person and the Priest on it, "" if none
	Mood           float64  // its Room Mood as a bar, 0 (Closed) .. 1
}

func (g *Game) card(i int) roomCard {
	r := g.hotel.Rooms[i]
	c := roomCard{Title: r.Number + " " + strings.ToUpper(r.Type.Beds), Boarded: !g.isOwned(i), Mirror: !r.Mirror.Empty(),
		Mood: float64(g.roomMood(i)-moodMin) / (moodMax - moodMin)}
	for _, ch := range g.changes {
		if ch.room == i && ch.to.Beds != r.Type.Beds {
			c.Change = strings.ToUpper(ch.to.Beds)
		}
	}
	for _, p := range g.parties {
		for _, guest := range p.Guests {
			switch {
			case p.Occupies(i, Human):
				c.Humans = append(c.Humans, guest.Character)
			case p.Occupies(i, Ghost):
				c.Ghosts = append(c.Ghosts, guest.Character)
			}
		}
	}
	if i < len(g.shades) && len(g.shades[i]) > 0 {
		c.Shadow = g.shades[i][0].name
	}
	for _, pr := range g.priests {
		if pr.RoomIndex == i && !pr.done {
			c.Priest = pr.Character
		}
	}
	return c
}

// changeFor: the Room's change still waiting for it to empty, or nil.
func (g *Game) changeFor(room int) *change {
	for _, c := range g.pending() {
		if c.room == room {
			return c
		}
	}
	return nil
}

// buyNow pays for item i for the Room at once, past the cart, and sends it: "" or why not.
func (g *Game) buyNow(i, room int) string {
	cart := g.cart
	g.cart = nil
	why := g.addToCart(i, room)
	if why == "" {
		why = g.placeOrder()
	}
	g.cart = cart
	return why
}

// updateRooms is the ROOMS tab's keys; false when the key is the store's own (A/D on the tab).
func (g *Game) updateRooms() bool {
	// game.go's ESC clears g.placing and keeps the store open while it is set: it stands for "inside the grid".
	if g.placing < 0 && g.termDepth > 0 {
		g.termDepth--
		g.message = ""
	}
	was := g.termDepth
	g.roomKeys()
	g.placing = g.termDepth - 1
	return was > 0 || g.termDepth > 0
}

func (g *Game) roomKeys() {
	key := g.in.JustPressed
	n, room := len(g.hotel.Rooms), g.termCursor
	cols, _ := g.hotel.Grid()
	rows := tabItems(tabRooms)
	switch g.termDepth {
	case 0:
		if key(ebiten.KeyW) || key(ebiten.KeyS) || key(ebiten.KeySpace) {
			g.termDepth, g.termCursor = 1, min(room, n-1)
		}
	case 1:
		switch {
		case key(ebiten.KeyA):
			g.termCursor = (room + n - 1) % n
		case key(ebiten.KeyD):
			g.termCursor = (room + 1) % n
		case key(ebiten.KeyW) && room >= cols:
			g.termCursor -= cols
		case key(ebiten.KeyW): // up from the top row: back on the tabs, as ESC
			g.termDepth = 0
		case key(ebiten.KeyS) && room+cols < n:
			g.termCursor += cols
		case key(ebiten.KeySpace):
			g.termDepth, g.termPick = 2, 0
		default:
			return
		}
		g.message = ""
	case 2:
		switch {
		case key(ebiten.KeyW):
			g.termPick, g.message = (g.termPick+len(rows)-1)%len(rows), ""
		case key(ebiten.KeyS):
			g.termPick, g.message = (g.termPick+1)%len(rows), ""
		case key(ebiten.KeySpace):
			i := rows[g.termPick]
			c := g.changeFor(room)
			switch {
			case shop[i].ID == rowChange && c != nil: // the same row calls a waiting change off
				g.cancelChange(c)
				g.message, g.termDepth = "CHANGE CANCELLED: "+price(c.paid)+" BACK", 1
			case shop[i].ID == rowChange && g.isOwned(room) && !g.changing(room): // its beds next, each with its fee
				if g.termDepth, g.message = 3, ""; !g.bedOpen(g.termBeds) {
					g.termBeds = 0
				}
			default:
				g.pay1(i, room)
			}
		}
	case 3:
		switch {
		case key(ebiten.KeyW):
			g.pickBeds(-1)
		case key(ebiten.KeyS):
			g.pickBeds(1)
		case key(ebiten.KeySpace):
			g.pay1(rows[g.termPick], room)
		}
	}
}

// pay1 buys the action for the Room and says so; bought, it is back on the grid.
func (g *Game) pay1(i, room int) {
	if g.message = g.buyNow(i, room); g.message == "" {
		g.message, g.termDepth = strings.ToUpper(shop[i].Name)+" FOR ROOM "+g.hotel.Rooms[room].Number+": PAID", 1
	}
}

func (g *Game) drawRooms(s *ebiten.Image) {
	room := g.termCursor
	if g.termDepth < 2 { // the hotel, a card a Room
		cols, rows := g.hotel.Grid()
		gw, gh := winW-32, paneH
		tw, th := (gw-(cols-1)*tileGap)/cols, (gh-(rows-1)*tileGap)/rows
		for i := range g.hotel.Rooms {
			x, y := listX+i%cols*(tw+tileGap), bodyY+i/cols*(th+tileGap)
			if g.termDepth == 1 && i == room {
				rect(s, x-3, y-3, tw+6, th+6, assets.Accent)
			}
			g.drawCard(s, i, x, y, tw, th)
		}
		return
	}
	if g.termDepth == 3 { // the beds to change to, each with its fee
		n := 0
		for k, b := range bedKinds {
			if g.bedOpen(k) {
				a := float32(1)
				if !g.afford(g.bedFee(k)) {
					a = 0.5
				}
				g.drawRowAt(s, n, k == g.termBeds, Item{Icon: "icon_bed_" + strings.ToLower(b.Beds)}, strings.ToUpper(b.Beds), g.bedFee(k), "", a)
				n++
			}
		}
	} else {
		for k, i := range tabItems(tabRooms) {
			it, name, c, word, a := shop[i], strings.ToUpper(shop[i].Name), g.priceOf(i), "", float32(1)
			w := g.changeFor(room)
			switch {
			case !g.onSale(i):
				a, word = 0.35, "SOON"
			case it.unlock() && g.unlocked(it.ID):
				a, word = 0.5, "OWNED"
			case it.ID == rowChange && w != nil:
				name, c = "CANCEL > "+strings.ToUpper(w.to.Beds), w.paid
			case it.ID == rowChange:
				if c = (Cost{}); !g.isOwned(room) || g.changing(room) { // its fee comes with the beds
					a = 0.5
				}
			case it.Refuse(g, room) != "" || g.coming(it.ID, room) || !g.afford(c):
				a = 0.5
			}
			g.drawRowAt(s, k, k == g.termPick, it, name, c, word, a)
		}
	}
	g.drawCard(s, room, paneX, bodyY, paneW, paneH)
}

// drawCard draws a Room's card in the box: number and type, a change on its way, boarded or its Parties'
// heads (humans, then ghosts in their ghost color, the Shadow black, the Priest), its Mirror, its Mood bar.
func (g *Game) drawCard(s *ebiten.Image, i, x, y, w, h int) {
	c := g.card(i)
	bg := assets.Floor
	if c.Boarded {
		bg = assets.Ink
	}
	rect(s, x, y, w, h, bg)
	g.tintedAt(s, c.Title, x+8, y+2, 2, assets.Ghost, 1)
	if c.Change != "" {
		g.tintedAt(s, "> "+c.Change, x+8, y+28, 1, assets.Accent, 1)
	}
	if c.Mirror {
		g.drawIcon(s, Item{Icon: "icon_mirror"}, x+w-14, y+20, 32, 1)
	}
	if c.Boarded {
		g.drawIcon(s, Item{Icon: "door_boarded"}, x+w/2, y+h/2+8, min(64, h-48), 1)
		return
	}
	hx, hy := x+8, y+44
	g.drawHeads(s, hx, hy, c.Humans...)
	if c.Priest != "" {
		g.drawHeads(s, hx+len(c.Humans)*(head.Dx()+2)+6, hy, c.Priest)
	}
	hy += head.Dy() + 4
	for _, name := range c.Ghosts {
		t := ghostTints[strings.Split(name, "#")[0]]
		if t == nil {
			t = []float32{.78, .94, 1} // assets.Ghost
		}
		op := &ebiten.DrawImageOptions{}
		op.GeoM.Translate(float64(hx), float64(hy))
		op.ColorScale.Scale(t[0], t[1], t[2], 1)
		s.DrawImage(g.sheet(name)[0].SubImage(head).(*ebiten.Image), op)
		hx += head.Dx() + 2
	}
	if c.Shadow != "" {
		g.drawHead(s, hx+4, hy, c.Shadow, 0)
	}
	ink := assets.Accent // the Mood bar: no number (show, don't tell)
	if c.Mood < 0.5 {
		ink = assets.Locked
	}
	rect(s, x+8, y+h-12, w-16, 6, assets.Ink)
	rect(s, x+8, y+h-12, int(float64(w-16)*c.Mood), 6, ink)
}
