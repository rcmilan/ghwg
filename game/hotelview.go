package game

import (
	"ghosthotel/assets"
	"image"
	"image/color"
	"slices"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
)

// The hotel view (07 U1, replacing T1's ROOMS tab): SPACE, wherever it has nothing else to do (U2), stops the world and pulls the camera
// back to the whole hotel. The Rooms themselves are the overview; WASD moves a highlight Room to Room and
// SPACE opens that Room's actions, each paid at once, no cart (a change of type takes a third SPACE for its
// beds). g.termDepth is how far in: 1 the Rooms, 2 a Room's actions, 3 the beds of its change. A/D steps
// back one level; ESC closes it, in one press from any depth.

const (
	viewFrames = 20 // the pull-back, each way
	viewPad    = 28 // screen px kept free around the whole hotel
	screenW    = viewW * zoom
	screenH    = viewH * zoom
)

func (g *Game) openView() {
	g.state, g.termDepth = StateView, 1
	g.refuse("")
	g.termCursor = min(g.termCursor, len(g.hotel.Rooms)-1)
	for i, r := range g.hotel.Rooms { // the highlight starts on the Room you stand in
		if g.feet().In(r.Shell()) {
			g.termCursor = i
		}
	}
}

// DevView is main's -view: the hotel view fully pulled back on a Room, its actions open if open.
func (g *Game) DevView(room int, open bool) {
	g.openView()
	if g.termCursor, g.viewT = room, 1; open {
		g.termDepth, g.termPick = 2, 0
	}
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

// roomRows: the Room's actions in the hotel view, shop indexes in the catalog's order: the key alone for a Room
// not bought, and never the key for one that is (2026-11-14). "Change type" is left
// out while there is nothing to change to (2026-11-14): one bed layout unlocked, no change waiting to call off,
// and no in-Room Mirror the Room could get by changing to the beds it has.
func (g *Game) roomRows(room int) []int {
	open := 0
	for k := range bedKinds {
		if g.bedOpen(k) {
			open++
		}
	}
	mirror := g.unlocked(RoomMirror) && g.hotel.Rooms[room].Mirror.Empty()
	return slices.DeleteFunc(tabItems(tabRooms), func(i int) bool {
		switch key := shop[i].ID == rowKey; {
		case !g.isOwned(room): // boarded up: its key, nothing else
			return !key
		case key: // bought: no key to buy
			return true
		}
		return shop[i].ID == rowChange && open < 2 && !mirror && g.changeFor(room) == nil
	})
}

// updateView is the hotel view's keys.
func (g *Game) updateView() {
	key := g.in.JustPressed
	n, room := len(g.hotel.Rooms), g.termCursor
	cols, _ := g.hotel.Grid()
	rows := g.roomRows(room)
	back := key(ebiten.KeyA) || key(ebiten.KeyD)
	switch g.termDepth {
	case 1:
		switch {
		case key(ebiten.KeyA):
			g.termCursor = (room + n - 1) % n
		case key(ebiten.KeyD):
			g.termCursor = (room + 1) % n
		case key(ebiten.KeyW) && room >= cols:
			g.termCursor -= cols
		case key(ebiten.KeyS) && room+cols < n:
			g.termCursor += cols
		case key(ebiten.KeySpace):
			g.termDepth, g.termPick = 2, 0
		default:
			return
		}
		g.refuse("")
	case 2:
		switch {
		case back:
			g.termDepth = 1
			g.refuse("")
		case key(ebiten.KeyW):
			g.termPick = (g.termPick + len(rows) - 1) % len(rows)
			g.refuse("")
		case key(ebiten.KeyS):
			g.termPick = (g.termPick + 1) % len(rows)
			g.refuse("")
		case key(ebiten.KeySpace):
			i := rows[g.termPick]
			c := g.changeFor(room)
			switch {
			case shop[i].ID == rowChange && c != nil: // the same row calls a waiting change off
				g.cancelChange(c)
				g.termDepth = 1
				g.refuse("") // the fee coming back shows on the Money counter
			case shop[i].ID == rowChange && g.isOwned(room) && !g.changing(room): // its beds next, each with its fee
				g.refuse("")
				if g.termDepth = 3; !g.bedOpen(g.termBeds) {
					g.termBeds = 0
				}
			default:
				g.pay1(i, room)
			}
		}
	case 3:
		switch {
		case back:
			g.termDepth = 2
			g.refuse("")
		case key(ebiten.KeyW):
			g.pickBeds(-1)
		case key(ebiten.KeyS):
			g.pickBeds(1)
		case key(ebiten.KeySpace):
			g.pay1(rows[g.termPick], room)
		}
	}
}

// pay1 buys the action for the Room, or says why not; bought, the highlight is back on the Rooms.
func (g *Game) pay1(i, room int) {
	why := g.buyNow(i, room)
	if why == "" { // no confirmation: the counter rolls and the courier walks in (show, don't tell)
		g.termDepth = 1
	}
	g.refuse(why)
}

// viewAt: the pulled-back camera at g.viewT. A world px p shows at p*k + (tx,ty) screen px; at viewT 0 that is
// the play camera, at 1 the whole hotel centered.
func (g *Game) viewAt() (k, tx, ty float64) {
	mw, mh := g.mapSize()
	ox, oy := g.cam.Origin(g.player.X, g.player.Y, mw, mh)
	fit := min(1, float64(screenW-2*viewPad)/float64(mw*zoom), float64(screenH-2*viewPad)/float64(mh*zoom))
	t := g.viewT * g.viewT * (3 - 2*g.viewT)
	k = zoom * (1 + (fit-1)*t)
	return k, -float64(ox)*(1-t) + (screenW-float64(mw*zoom)*fit)/2*t, -float64(oy)*(1-t) + (screenH-float64(mh*zoom)*fit)/2*t
}

// drawView draws the world pulled back by g.viewT and, once the view is open, the Rooms' labels, the
// highlight and the picked Room's actions.
func (g *Game) drawView(screen *ebiten.Image, bg color.Color) {
	mw, mh := g.mapSize()
	if g.wide == nil || g.wide.Bounds().Size() != image.Pt(mw*zoom, mh*zoom) {
		g.wide = ebiten.NewImage(mw*zoom, mh*zoom)
	}
	g.wide.Fill(bg)
	fog := g.fog // the view sees into every Room: the veils lift as it pulls back
	g.fog = make([]float64, len(fog))
	for i, f := range fog {
		g.fog[i] = f * (1 - g.viewT)
	}
	g.drawWorld(g.wide, 0, 0)
	g.fog = fog
	k, tx, ty := g.viewAt()
	op := &ebiten.DrawImageOptions{Filter: ebiten.FilterLinear}
	op.GeoM.Scale(k/zoom, k/zoom)
	op.GeoM.Translate(tx, ty)
	if g.state == StateGrow {
		g.drawGrown(screen, k, tx, ty, bg)
	} else {
		screen.DrawImage(g.wide, op)
	}
	if g.state != StateView {
		return
	}
	on := func(r image.Rectangle) image.Rectangle { // world px to screen px
		return image.Rect(int(float64(r.Min.X)*k+tx), int(float64(r.Min.Y)*k+ty), int(float64(r.Max.X)*k+tx), int(float64(r.Max.Y)*k+ty))
	}
	for i, room := range g.hotel.Rooms {
		r := on(room.Shell())
		if g.boarded(i) { // a Room not bought yet lies under the fog (2026-11-14): the ones to buy stand out
			n := assets.Night
			rect(screen, r.Min.X, r.Min.Y, r.Dx(), r.Dy(), color.NRGBA{n.R, n.G, n.B, uint8(255 * fogAlpha * g.viewT)})
		}
		label := room.Number
		if c := g.changeFor(i); c != nil {
			label += " > " + typeLabel(c.to)
		}
		rect(screen, r.Min.X, r.Min.Y-glyphH-2, len(label)*glyphW+8, glyphH+2, storeInk)
		g.tinted(screen, label, r.Min.X+4, r.Min.Y-glyphH-1, storeBarInk, 1)
	}
	r := on(g.hotel.Rooms[g.termCursor].Shell())
	const b = 3
	for _, e := range [...]image.Rectangle{{r.Min, image.Pt(r.Max.X, r.Min.Y+b)}, {image.Pt(r.Min.X, r.Max.Y-b), r.Max}, {r.Min, image.Pt(r.Min.X+b, r.Max.Y)}, {image.Pt(r.Max.X-b, r.Min.Y), r.Max}} {
		rect(screen, e.Min.X, e.Min.Y, e.Dx(), e.Dy(), storeAccent)
	}
	if g.termDepth >= 2 {
		g.drawActions(screen, r)
	}
	help := [...]string{"", "WASD ROOM   SPACE OPEN   ESC CLOSE", "W/S ACTION   SPACE BUY NOW   A/D BACK   ESC CLOSE", "W/S BEDS   SPACE BUY NOW   A/D BACK   ESC CLOSE"}[g.termDepth]
	x := screenW - 16 - len(help)*glyphW
	rect(screen, x-6, screenH-22, len(help)*glyphW+12, glyphH+2, storeInk)
	g.tinted(screen, help, x, screenH-21, storeBarInk, 1)
}

// drawActions draws the picked Room's actions, or the beds of its change, as store rows on a card beside the
// Room's screen box r.
func (g *Game) drawActions(screen *ebiten.Image, r image.Rectangle) {
	if g.panel == nil {
		g.panel = ebiten.NewImage(screenW, screenH) // the rows draw at the store's own list spot; the card is moved
	}
	room := g.termCursor
	type line struct {
		picked bool
		it     Item
		name   string
		c      Cost
		word   string
		a      float32
	}
	var lines []line
	if g.termDepth == 3 { // the beds to change to, each with its fee
		for k, bed := range bedKinds {
			if g.bedOpen(k) {
				a := float32(1)
				if !g.afford(g.bedFee(k)) {
					a = 0.5
				}
				lines = append(lines, line{k == g.termBeds, Item{Icon: "icon_bed_" + strings.ToLower(bed.Beds)}, strings.ToUpper(bed.Beds), g.bedFee(k), "", a})
			}
		}
	} else {
		for k, i := range g.roomRows(room) {
			it, name, c, word, a := shop[i], strings.ToUpper(shop[i].Name), g.priceOf(i), "", float32(1)
			w := g.changeFor(room)
			switch {
			case !g.onSale(i):
				a, word = 0.35, "SOON"
			case it.unlock() && g.unlocked(it.ID):
				a, word = 0.5, "OWNED"
			case it.ID == rowChange && w != nil:
				name, c = "CANCEL > "+typeLabel(w.to), w.paid
			case it.ID == rowChange:
				if c = (Cost{}); !g.isOwned(room) || g.changing(room) { // its fee comes with the beds
					a = 0.5
				}
			case it.Refuse != nil && it.Refuse(g, room) != "" || g.coming(it.ID, room) || !g.afford(c):
				a = 0.5
			}
			lines = append(lines, line{k == g.termPick, it, name, c, word, a})
		}
	}
	const head = 34
	box := image.Rect(listX-8, bodyY-head-8, listX+listW+8, bodyY+len(lines)*rowH+4)
	g.panel.Clear()
	rect(g.panel, box.Min.X, box.Min.Y, box.Dx(), box.Dy(), storeInk)
	rect(g.panel, box.Min.X+3, box.Min.Y+3, box.Dx()-6, box.Dy()-6, storePage)
	rect(g.panel, box.Min.X+3, box.Min.Y+3, box.Dx()-6, head, storeBar)
	title := "ROOM " + g.hotel.Rooms[room].Number + "  " + typeLabel(g.hotel.Rooms[room].Type)
	g.tintedAt(g.panel, title, listX, box.Min.Y+4, 2, storeBarInk, 1)
	for k, l := range lines {
		g.drawRowAt(g.panel, k, l.picked, l.it, l.name, l.c, l.word, l.a)
	}
	x := r.Max.X + 10 // beside the Room, on the side with space
	if x+box.Dx() > screenW-4 {
		x = max(4, r.Min.X-10-box.Dx())
	}
	y := min(max(r.Min.Y, 44), screenH-28-box.Dy())
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Translate(float64(x), float64(y)) // a SubImage draws from its own corner
	screen.DrawImage(g.panel.SubImage(box).(*ebiten.Image), op)
}

// The pause (07 U1): ESC with nothing open stops the world on Resume / Quit to title. ESC or SPACE on Resume
// goes on; only SPACE on Quit ends the Run.
var pauseRows = [...]string{"RESUME", "QUIT TO TITLE"}

func (g *Game) updatePause() {
	key := g.in.JustPressed
	switch {
	case key(ebiten.KeyW) || key(ebiten.KeyS):
		g.pausePick = 1 - g.pausePick
	case key(ebiten.KeySpace) && g.pausePick == 1:
		g.toTitle()
	case key(ebiten.KeySpace):
		g.state = StatePlay
	}
}

func (g *Game) drawPause(s *ebiten.Image) {
	rect(s, 0, 0, screenW, screenH, color.RGBA{0, 0, 0, 150})
	const w, h = 320, 150
	x, y := (screenW-w)/2, (screenH-h)/2
	rect(s, x, y, w, h, storeInk)
	rect(s, x+3, y+3, w-6, h-6, storePage)
	g.tintedAt(s, "PAUSED", x+w/2-6*glyphW, y+10, 2, storeInk, 1)
	for k, name := range pauseRows {
		ry := y + 56 + k*rowH
		if k == g.pausePick {
			rect(s, x+12, ry, w-24, rowH-4, storeCard)
			rect(s, x+12, ry, 4, rowH-4, storeAccent)
		}
		g.tintedAt(s, name, x+w/2-len(name)*glyphW, ry+2, 2, storeInk, 1)
	}
}
