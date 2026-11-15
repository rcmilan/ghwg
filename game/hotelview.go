package game

import (
	"fmt"
	"ghosthotel/assets"
	"ghosthotel/world"
	"image"
	"image/color"
	"math"
	"slices"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
)

// The hotel view (07 U1, 08 F0): SPACE, wherever it has nothing else to do (U2), pulls the camera back to the
// whole hotel while the world runs on in slow motion (viewSlow). g.termDepth is how far in: 1 the hotel, no Room
// picked; 2 a Room picked (WASD), the camera part way toward it; 3 its ring of actions (SPACE), the camera all the
// way in; 4 the inner ring of the beds a change of type can take. Each action is paid at once, no cart. ESC
// steps back one level, out of the view from 1 (game.go).

const (
	viewFrames  = 20 // the pull-back, each way
	viewPad     = 28 // screen px kept free around the whole hotel
	screenW     = viewW * zoom
	screenH     = viewH * zoom
	ringZoom    = 1.8  // screen px per world px with a Room's ring open: the Room and its ring fit the screen
	viewLerp    = 0.15 // the camera's ease toward a picked Room, each frame
	shakeFrames = 12   // the "no" shake of an action SPACE can't buy (2026-11-15, grill 3)
)

func (g *Game) openView() {
	g.state, g.termDepth = StateView, 1
	g.refuse("")
	g.termCursor = min(g.termCursor, len(g.hotel.Rooms)-1)
	for i, r := range g.hotel.Rooms { // the Room WASD picks first is the one you stand in
		if g.feet().In(r.Shell()) {
			g.termCursor = i
		}
	}
}

// DevView is main's -view: the hotel view pulled back, no Room picked (depth -1), the Room picked (0), its ring
// open on its pick-th action clockwise from the top (1), or its Change type's beds on the pick-th (2).
func (g *Game) DevView(room, depth, pick int) {
	g.openView()
	g.termCursor, g.viewT, g.termDepth = room, 1, min(2+max(depth, -1), 4)
	g.viewIn, g.termPick = viewIns[g.termDepth], 0
	if rows := g.roomRows(room); g.termDepth > 2 && len(rows) > 0 {
		g.termPick = ringSlot[shop[rows[min(pick, len(rows)-1)]].ID]
		if beds := g.changeBeds(room); g.termDepth == 4 && len(beds) > 0 {
			g.termPick, g.termBeds = 0, beds[min(pick, len(beds)-1)]
		}
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

// ringSlot: each Room action's fixed place on its ring (2026-11-15), clockwise from the top: Change type (a
// boarded Room's key), Deluxe on the right, the Room mirror (Add mirror once it is bought) lower right, Father M lower left, Father K on the left.
var ringSlot = map[Unlock]int{rowKey: 0, rowChange: 0, rowDeluxe: 1, RoomMirror: 2, rowAddMirror: 2, rowFatherM: 3, rowFatherK: 4}

const ringSlots = 5

// roomRows: the Room's actions, shop indexes clockwise from the top, only the ones it can take now (2026-11-15):
// a boarded Room's key alone, never a bought one's; Change type while it has a layout to change to, or a change
// waiting to call off; a Father's call only while he is nowhere in the hotel. One the wallet can't pay stays.
func (g *Game) roomRows(room int) []int {
	rows := slices.DeleteFunc(tabItems(tabRooms), func(i int) bool {
		it := shop[i]
		switch {
		case (it.ID == rowKey) == g.isOwned(room):
			return true
		case it.ID == rowChange:
			return g.changeFor(room) == nil && (g.changing(room) || len(g.changeBeds(room)) == 0)
		case (it.ID == rowFatherK || it.ID == rowFatherM) && g.fatherIn(it.ID == rowFatherM):
			return true
		}
		return !g.onSale(i) || g.owned(i) || it.Refuse != nil && it.Refuse(g, room) != "" || g.coming(it.ID, room)
	})
	slices.SortFunc(rows, func(a, b int) int { return ringSlot[shop[a].ID] - ringSlot[shop[b].ID] })
	return rows
}

// ring: the Room's actions by slot, -1 in an empty one.
func (g *Game) ring(room int) [ringSlots]int {
	r := [ringSlots]int{-1, -1, -1, -1, -1}
	for _, i := range g.roomRows(room) {
		r[ringSlot[shop[i].ID]] = i
	}
	return r
}

// turnRing: the next action from slot s, clockwise (d 1) or back (-1), past the empty slots; d 0 stays on s if it
// holds one. -1 for an empty ring.
func turnRing(r [ringSlots]int, s, d int) int {
	if d == 0 {
		if r[s] >= 0 {
			return s
		}
		d = 1
	}
	for range ringSlots {
		if s = (s + d + ringSlots) % ringSlots; r[s] >= 0 {
			return s
		}
	}
	return -1
}

// ringStart: where the Room's ring opens: its top, or the next action clockwise; past calling a change off, which
// comes last (2026-11-14, stray cancel), unless it is alone.
func (g *Game) ringStart(room int) int {
	r := g.ring(room)
	if r[0] >= 0 && shop[r[0]].ID == rowChange && g.changeFor(room) != nil {
		return turnRing(r, 0, 1)
	}
	return turnRing(r, 0, 0)
}

// viewIns: the camera's way in from the whole hotel to the picked Room, by g.termDepth.
var viewIns = [...]float64{0, 0, 0.5, 1, 1}

// updateView runs the world on, slowed, and reads the hotel view's keys.
func (g *Game) updateView() {
	for left := g.scale * viewSlow; left > 0; left-- {
		g.advance(0, 0, min(left, 1))
	}
	g.viewIn += (viewIns[g.termDepth] - g.viewIn) * viewLerp
	g.shake = max(0, g.shake-1)
	key := g.in.JustPressed
	n, room := len(g.hotel.Rooms), g.termCursor
	cols, _ := g.hotel.Grid()
	r := g.ring(room)
	if g.termDepth >= 3 { // the world ran on: the focus moves on clockwise from an action gone, out of a ring left empty
		if g.termPick = turnRing(r, g.termPick, 0); g.termPick < 0 {
			g.termDepth = 2
		}
	}
	beds := g.changeBeds(room)
	if g.termDepth == 4 && (g.termPick != 0 || shop[r[0]].ID != rowChange || g.changeFor(room) != nil || len(beds) == 0) {
		g.termDepth = 3
	}
	turn := 0 // the rings turn: D or S clockwise, A or W back
	switch {
	case key(ebiten.KeyD) || key(ebiten.KeyS):
		turn = 1
	case key(ebiten.KeyA) || key(ebiten.KeyW):
		turn = -1
	}
	switch g.termDepth {
	case 1:
		if turn != 0 { // WASD picks a Room: the last one picked, or the one you stand in
			g.termDepth = 2
		}
	case 2:
		switch {
		case key(ebiten.KeyA):
			g.termCursor = (room + n - 1) % n
		case key(ebiten.KeyD):
			g.termCursor = (room + 1) % n
		case key(ebiten.KeyW) && room >= cols:
			g.termCursor -= cols
		case key(ebiten.KeyS) && room+cols < n:
			g.termCursor += cols
		case key(ebiten.KeySpace) && g.ringStart(room) >= 0:
			g.termDepth, g.termPick = 3, g.ringStart(room)
		}
		g.refuse("")
	case 3:
		g.termPick = turnRing(r, g.termPick, turn)
		if !key(ebiten.KeySpace) {
			break
		}
		switch i, c := r[g.termPick], g.changeFor(room); {
		case shop[i].ID == rowChange && c != nil:
			g.cancelChange(c) // the fee coming back shows on the Money counter
			g.termDepth = 2
			g.refuse("")
		case shop[i].ID == rowChange:
			g.termDepth, g.termBeds = 4, beds[0]
		default:
			g.pay1(i, room)
		}
	case 4:
		k := max(0, slices.Index(beds, g.termBeds))
		g.termBeds = beds[(k+turn+len(beds))%len(beds)]
		if key(ebiten.KeySpace) {
			g.pay1(r[0], room)
		}
	}
}

// pay1 buys the action for the Room, or shakes no; bought, the view is back on the Room.
func (g *Game) pay1(i, room int) {
	why := g.buyNow(i, room)
	if why == "" { // no confirmation: the counter rolls and the courier walks in (show, don't tell)
		g.termDepth = 2
	} else {
		g.shake = shakeFrames
	}
	g.refuse(why)
}

// viewAt: the pulled-back camera at g.viewT. A world px p shows at p*k + (tx,ty) screen px; at viewT 0 that is
// the play camera, at 1 the hotel view's: the whole hotel centered, eased (g.viewIn) toward the picked Room
// centered at ringZoom.
func (g *Game) viewAt() (k, tx, ty float64) {
	mw, mh := g.mapSize()
	ox, oy := g.cam.Origin(g.player.X, g.player.Y, mw, mh)
	fit := min(1, float64(screenW-2*viewPad)/float64(mw*zoom), float64(screenH-2*viewPad)/float64(mh*zoom))
	in := g.viewIn
	if g.growing != nil {
		in = 0
	}
	c := g.hotel.Rooms[min(g.termCursor, len(g.hotel.Rooms)-1)].Shell()
	kv := zoom*fit + (ringZoom-zoom*fit)*in
	px := float64(mw)/2 + (float64(c.Min.X+c.Max.X)/2-float64(mw)/2)*in
	py := float64(mh)/2 + (float64(c.Min.Y+c.Max.Y)/2-float64(mh)/2)*in
	t := g.viewT * g.viewT * (3 - 2*g.viewT)
	k = zoom + (kv-zoom)*t
	return k, -float64(ox)*(1-t) + (screenW/2-px*kv)*t, -float64(oy)*(1-t) + (screenH/2-py*kv)*t
}

// drawView draws the world pulled back by g.viewT and, once the view is open, the Rooms' labels and the picked
// Room's ring.
func (g *Game) drawView(screen *ebiten.Image, bg color.Color) {
	mw, mh := g.mapSize()
	g.wide = reuse(&g.wideBuf, g.wide, image.Pt(mw*zoom, mh*zoom))
	if gr := g.growing; gr == nil || gr.drawn != dimName(g.ghost) || g.viewT < 1 { // pulled back, a growth's slide only moves the hotel, which waits: drawn once (08 W3 browser profile); the warm-up draws the other dimension
		g.wide.Fill(bg)
		fog := g.fog // the view sees into every Room: the veils lift as it pulls back
		g.fog = make([]float64, len(fog))
		for i, f := range fog {
			g.fog[i] = f * (1 - g.viewT)
		}
		g.drawWorld(g.wide, 0, 0)
		if g.growing != nil {
		}
		g.fog = fog
		if gr != nil && g.viewT == 1 { // pulled back: the veils are lifted
			gr.drawn = dimName(g.ghost)
		}
	}
	k, tx, ty := g.viewAt()
	op := &ebiten.DrawImageOptions{Filter: ebiten.FilterLinear}
	op.GeoM.Scale(k/zoom, k/zoom)
	op.GeoM.Translate(tx, ty)
	if g.state == StateGrow {
		g.drawGrown(screen, k, tx, ty, bg)
	} else {
		screen.DrawImage(g.wide, op)
	}
	pills := func() { g.drawRequests(screen, -int(math.Round(tx)), -int(math.Round(ty)), k) } // at the size they have in play
	if g.state != StateView {
		if g.state != StateGrow {
			pills()
		}
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
		if c, ok := g.pairInk(i); ok { // the fit (07 X2): red a bad pair, the Ectoplasm ink a good one (match.go)
			g.drawPair(screen, i, c, k, tx, ty)
		}
		label := room.Number
		if c := g.changeFor(i); c != nil {
			label += " > " + typeLabel(c.to)
		}
		rect(screen, r.Min.X, r.Min.Y-glyphH-2, len(label)*glyphW+8, glyphH+2, storeInk)
		g.tinted(screen, label, r.Min.X+4, r.Min.Y-glyphH-1, storeBarInk, 1)
	}
	pills() // over the veils and labels: a pill is never cut by a Room beside it
	if g.termDepth >= 3 {
		g.drawRing(screen, on(g.hotel.Rooms[g.termCursor].Shell()))
	}
	help := [...]string{"", "WASD PICK A ROOM   ESC CLOSE", "WASD ROOM   SPACE ACTIONS   ESC BACK", "WASD TURN   SPACE BUY   ESC BACK", "WASD BEDS   SPACE BUY   ESC BACK"}[g.termDepth]
	x := screenW - 16 - len(help)*glyphW
	rect(screen, x-6, screenH-22, len(help)*glyphW+12, glyphH+2, storeInk)
	g.tinted(screen, help, x, screenH-21, storeBarInk, 1)
}

// drawRing draws the picked Room's ring around its screen box r (2026-11-15): an icon in each action's slot, the
// focused one framed, and in the middle its name, price (red when the wallet can't pay it) and effect; with
// Change type open, its beds on a smaller ring inside.
func (g *Game) drawRing(screen *ebiten.Image, r image.Rectangle) {
	room, ring := g.termCursor, g.ring(g.termCursor)
	cx, cy := (r.Min.X+r.Max.X)/2, (r.Min.Y+r.Max.Y)/2
	at := func(rad float64, k, n int) (int, int) { // the k-th of n places clockwise from the top
		a := 2 * math.Pi * float64(k) / float64(n)
		return cx + int(rad*math.Sin(a)), cy - int(rad*math.Cos(a))
	}
	slot := func(it Item, x, y int, focused bool, a float32) {
		h := 26
		if focused {
			h = 32
			rect(screen, x-h-3, y-h-3, 2*h+6, 2*h+6, storeAccent)
		}
		rect(screen, x-h, y-h, 2*h, 2*h, storeInk)
		rect(screen, x-h+2, y-h+2, 2*h-4, 2*h-4, storeCard)
		g.drawIcon(screen, it, x, y, 2*h-12, a)
	}
	inner := g.termDepth == 4
	for s, i := range ring {
		if i >= 0 {
			a := float32(1)
			if inner {
				a = 0.4
			}
			x, y := at(0.72*float64(r.Dx()), s, ringSlots)
			slot(shop[i], x, y, !inner && s == g.termPick, a)
		}
	}
	i := ring[g.termPick]
	it, name, c, effect := shop[i], strings.ToUpper(shop[i].Name), g.priceOf(i), shop[i].Blurb
	switch w := g.changeFor(room); {
	case inner:
		beds := g.changeBeds(room)
		for k, b := range beds {
			x, y := at(0.36*float64(r.Dx()), k, len(beds))
			slot(Item{Icon: "icon_bed_" + strings.ToLower(bedKinds[b].Beds)}, x, y, b == g.termBeds, 1)
		}
		t, _ := world.TypeIn(g.types, g.hotel.Rooms[room].Type.Name, bedKinds[g.termBeds].Beds)
		name, c = strings.ToUpper(bedKinds[g.termBeds].Beds), g.bedFee(g.termBeds)
		effect = fmt.Sprintf("Sleeps %d. $%d a night, once it's empty.", len(t.Stays), t.Rate)
	case it.ID == rowChange && w != nil:
		name, c, effect = "CANCEL > "+typeLabel(w.to), w.paid, "Calls the change off. Full refund."
	case it.ID == rowChange: // its fee comes with the beds
		c = Cost{}
	}
	red := !g.afford(c) && (inner || it.ID != rowChange)
	w := max(len(name)*glyphW*2, len(effect)*glyphW, priceW(c, 2)) + 24
	x := cx - w/2 + int(math.Sin(float64(g.shake))*float64(g.shake)/2)
	rect(screen, x, cy-44, w, 88, storeInk)
	rect(screen, x+2, cy-42, w-4, 84, storePage)
	g.tintedAt(screen, name, x+w/2-len(name)*glyphW, cy-38, 2, storeInk, 1)
	g.drawPrice(screen, c, x+w/2-priceW(c, 2)/2, cy-8, 2, red) // red: the Terminal's red (2026-11-15, grill 3)
	g.tinted(screen, effect, x+w/2-len(effect)*glyphW/2, cy+22, storeInk, 1)
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

// The picked Room's glow (live tests 3, 4): a radial glow from its centre in the Ectoplasm ink, breathing green
// to purple, on the floors around it only: drawn right after the floor (drawWorld), so every wall, prop and
// actor stands in front of it, and nothing outside the hotel takes it. The Room itself is cut out of it. It
// cues what the cursor is on; later, anything actionable. Placeholders: glowReach (radius, in Room widths) and
// glowAlpha (at the centre).
const (
	glowReach = 1.2
	glowAlpha = 1.0
)

var glowLayer *ebiten.Image // the floor's size, world px

// drawRoomGlow: op is the floor's (drawWorld).
func (g *Game) drawRoomGlow(s *ebiten.Image, op *ebiten.DrawImageOptions) {
	if g.state != StateView || g.termDepth < 2 { // no Room picked, none glows
		return
	}
	if glowLayer == nil || glowLayer.Bounds() != g.floorImg.Bounds() {
		glowLayer = ebiten.NewImage(g.floorImg.Bounds().Dx(), g.floorImg.Bounds().Dy())
	}
	glowLayer.Clear()
	r := g.hotel.Rooms[g.termCursor].Shell()
	rad := glowReach * float64(r.Dx())
	k := 2 * rad / float64(g.glow.Bounds().Dx()) // the Terminal's glow sprite (light.go)
	o := &ebiten.DrawImageOptions{Filter: ebiten.FilterLinear}
	o.GeoM.Scale(k, k)
	c := r.Min.Add(r.Max).Div(2)
	o.GeoM.Translate(float64(c.X)-rad, float64(c.Y)-rad)
	o.ColorScale.ScaleWithColor(resInk(Ecto))
	o.ColorScale.ScaleAlpha(glowAlpha * float32(g.viewT))
	glowLayer.DrawImage(g.glow, o)
	glowLayer.DrawImage(g.floorImg, &ebiten.DrawImageOptions{Blend: ebiten.BlendDestinationIn}) // floors only
	m := g.hotel.Map
	for y := range m.H { // nor the void around the hotel, a floor tile too
		for x := range m.W {
			if m.TS.Tiles[m.At(world.LayerFloor, x, y)].Name == "void" {
				o := m.Origin(x, y)
				glowLayer.SubImage(image.Rectangle{o, o.Add(image.Pt(world.TileSize, world.TileSize))}).(*ebiten.Image).Clear()
			}
		}
	}
	glowLayer.SubImage(r).(*ebiten.Image).Clear() // not the Room itself
	s.DrawImage(glowLayer, &ebiten.DrawImageOptions{GeoM: op.GeoM, Blend: ebiten.BlendLighter})
}

// dimName: "ghost" or "real".
func dimName(ghost bool) string {
	if ghost {
		return "ghost"
	}
	return "real"
}
