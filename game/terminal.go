package game

import (
	"fmt"
	"image"
	"image/color"
	"slices"
	"strings"

	"ghosthotel/assets"

	"ghosthotel/world"

	"github.com/hajimehoshi/ebiten/v2"
)

// openTerminal pauses the world on the Terminal (D73), on its store. ESC closes it; the cart stays.
func (g *Game) openTerminal() {
	g.state, g.message, g.placing = StateTerminal, "", -1
	g.termPick = min(g.termPick, g.rowCount()-1)
}

// tabItems: the shop rows of a catalog tab, in order. Every item is always listed (2026-11-08).
func tabItems(t Tab) []int {
	var rows []int
	for i, it := range shop {
		if it.Tab == t {
			rows = append(rows, i)
		}
	}
	return rows
}

// rowCount: rows in the open tab; the cart's lines, then the checkout, then the CHANGES list: the Rooms'
// changes of type still waiting, each cancellable (K1).
func (g *Game) rowCount() int {
	if g.termTab == tabCart {
		return len(g.cart) + 1 + len(g.pending())
	}
	return len(tabItems(g.termTab))
}

// updateTerminal (D180): A/D picks the tab, W/S the row, SPACE adds to the cart, removes a cart line,
// checks out or cancels a waiting change. A Service first asks for its Room: A/D picks it (W/S the bed layout
// of a change of type), SPACE adds, ESC backs out (game.go).
func (g *Game) updateTerminal() {
	key := g.in.JustPressed
	if g.placing >= 0 {
		n := len(g.hotel.Rooms)
		switch {
		case key(ebiten.KeyA):
			g.termCursor, g.message = (g.termCursor+n-1)%n, ""
		case key(ebiten.KeyD):
			g.termCursor, g.message = (g.termCursor+1)%n, ""
		case shop[g.placing].ID == rowChange && key(ebiten.KeyW):
			g.pickBeds(-1)
			g.message = ""
		case shop[g.placing].ID == rowChange && key(ebiten.KeyS):
			g.pickBeds(1)
			g.message = ""
		case key(ebiten.KeySpace):
			if g.message = g.addToCart(g.placing, g.termCursor); g.message == "" {
				g.message = strings.ToUpper(shop[g.placing].Name) + " FOR ROOM " + g.hotel.Rooms[g.termCursor].Number + " IN THE CART"
				g.placing = -1
			}
		}
		return
	}
	switch {
	case key(ebiten.KeyA):
		g.termTab, g.termPick, g.message = (g.termTab+tabs-1)%tabs, 0, ""
	case key(ebiten.KeyD):
		g.termTab, g.termPick, g.message = (g.termTab+1)%tabs, 0, ""
	case key(ebiten.KeyW):
		g.termPick, g.message = (g.termPick+g.rowCount()-1)%g.rowCount(), ""
	case key(ebiten.KeyS):
		g.termPick, g.message = (g.termPick+1)%g.rowCount(), ""
	case key(ebiten.KeySpace):
		g.message = g.press()
	}
}

// press is SPACE on the picked row: what the store says back.
func (g *Game) press() string {
	if g.termTab == tabCart {
		if g.termPick < len(g.cart) {
			g.removeLine(g.termPick)
			g.termPick = min(g.termPick, len(g.cart))
			return "REMOVED"
		}
		if k := g.termPick - len(g.cart) - 1; k >= 0 {
			c := g.pending()[k]
			g.cancelChange(c)
			g.termPick = min(g.termPick, g.rowCount()-1)
			return "CHANGE CANCELLED: " + price(c.paid) + " BACK"
		}
		if why := g.placeOrder(); why != "" {
			return why
		}
		g.termPick, g.state = 0, StatePlay // an order placed closes the store
		return ""
	}
	i := tabItems(g.termTab)[g.termPick]
	if it := shop[i]; it.room() && g.onSale(i) && !(it.Once && g.unlocked(it.ID)) && (g.affords(i) || it.ID == rowChange) { // a change's fee depends on the layout, picked next
		if g.placing = i; it.ID == rowChange && !g.bedOpen(g.termBeds) {
			g.termBeds = 0
		}
		return ""
	}
	if why := g.addToCart(i, -1); why != "" {
		return why
	}
	return strings.ToUpper(shop[i].Name) + " IN THE CART"
}

// ShopPrint opens the store holding money and ecto, every Hotel unlock still to buy: main's -shop dev
// start (T187).
func (g *Game) ShopPrint(money, ecto float64) {
	g.locked = map[Unlock]bool{}
	for _, it := range shop {
		if it.Tab == tabHotel {
			g.locked[it.ID] = true
		}
	}
	g.wallet = [2]Num{Num(money), Num(ecto)}
	g.openTerminal()
}

// Terminal layout, screen px: a browser window.
const (
	termX, termY, termW, termH = 120, 30, 720, 470
	winX, winY, winW           = termX + 8, termY + 8, termW - 16
	barH                       = 36                  // the address bar
	tabY, tabH                 = winY + barH + 8, 36 // the tabs
	bodyY                      = tabY + tabH + 10
	listX, listW, rowH         = winX + 8, 352, 38 // the item list
	paneX, paneW, paneH        = listX + listW + 12, winW - listW - 28, 300
	tileGap                    = 8
)

// head is a character's head on the idle sheet, facing down: the Terminal's size icon (D97, T010).
var head = image.Rect(18*charW, 9, 19*charW, 22)

// drawHeads draws one shrunk head per character, left to right from (x,y).
func (g *Game) drawHeads(s *ebiten.Image, x, y int, names ...string) {
	for i, name := range names {
		g.drawHead(s, x+i*(head.Dx()+2), y, name, 1)
	}
}

// drawHead draws a character's head at (x,y), its colors scaled by c: 0 is a Shadow's black (D169).
func (g *Game) drawHead(s *ebiten.Image, x, y int, name string, c float32) {
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Translate(float64(x), float64(y))
	op.ColorScale.Scale(c, c, c, 1)
	s.DrawImage(g.sheet(name)[0].SubImage(head).(*ebiten.Image), op)
}

// tileIcons: the icons made of the world's own tiles, rows of cells, so the store shows a thing as the
// hotel does (the wall Mirror, a Room's door, the Standard beds).
var tileIcons = map[string][][]string{
	"icon_mirror":     {{"prop_mirror_r0c0"}, {"prop_mirror_r1c0"}},
	"icon_door":       {{"door_plain_top"}, {"door_plain_bottom"}},
	"icon_bed_single": {{"prop_bed_white_single_r0c0"}, {"prop_bed_white_single_r1c0"}, {"prop_bed_white_single_r2c0"}},
	"icon_bed_twin": {{"prop_bed_white_single_r0c0", "prop_bed_white_single_r0c0"}, {"prop_bed_white_single_r1c0", "prop_bed_white_single_r1c0"},
		{"prop_bed_white_single_r2c0", "prop_bed_white_single_r2c0"}},
	"icon_bed_double": {{"prop_bed_white_r0c0", "prop_bed_white_r0c1"}, {"prop_bed_white_r1c0", "prop_bed_white_r1c1"}, {"prop_bed_white_r2c0", "prop_bed_white_r2c1"}},
}

// tileIcon builds one of tileIcons.
func (g *Game) tileIcon(rows [][]string) *ebiten.Image {
	const ts = world.TileSize
	img := ebiten.NewImage(len(rows[0])*ts, len(rows)*ts)
	for y, row := range rows {
		for x, name := range row {
			if id, ok := g.ts.ID(name); ok {
				op := &ebiten.DrawImageOptions{}
				op.GeoM.Translate(float64(x*ts), float64(y*ts))
				img.DrawImage(g.tiles[id], op)
			}
		}
	}
	return img
}

// icon: an item's picture, a sprite (A1, or one of tileIcons) or a character's head.
func (g *Game) icon(it Item) (*ebiten.Image, float32) {
	if it.Head {
		return g.sheet(it.Icon)[0].SubImage(head).(*ebiten.Image), 1
	}
	return g.sprite(it.Icon), 1 // nil if missing: drawn as nothing
}

// drawIcon fits an item's icon in a box px square centered at (cx,cy), at alpha a.
func (g *Game) drawIcon(s *ebiten.Image, it Item, cx, cy, box int, a float32) {
	img, c := g.icon(it)
	if img == nil {
		return
	}
	w, h := img.Bounds().Dx(), img.Bounds().Dy()
	k := float64(box / max(w, h)) // whole pixels
	if k == 0 {                   // bigger than the box: shrunk to fit
		k = float64(box) / float64(max(w, h))
	}
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Scale(k, k)
	op.GeoM.Translate(float64(cx)-float64(w)*k/2, float64(cy)-float64(h)*k/2)
	op.ColorScale.Scale(c*a, c*a, c*a, a)
	s.DrawImage(img, op)
}

// drawPrice draws "$N" in Money's ink and a skull with N in Ecto's, from (x,y); it returns the width.
func (g *Game) drawPrice(s *ebiten.Image, c Cost, x, y int, scale float64) int {
	if g.skull == nil {
		g.skull = bitmap(skullRows)
	}
	x0 := x
	if c[Money] > 0 {
		v := "$" + fmtNum(c[Money])
		g.tintedAt(s, v, x, y, scale, resInk(Money), 1)
		x += int(float64(len(v)*glyphW+glyphW) * scale)
	}
	if c[Ecto] > 0 {
		g.stamp(s, g.skull, x, y+int(skullDY*scale), scale, resInk(Ecto))
		x += int(float64(len(skullRows[0])+2) * scale)
		v := fmtNum(c[Ecto])
		g.tintedAt(s, v, x, y, scale, resInk(Ecto), 1)
		x += int(float64(len(v)*glyphW) * scale)
	}
	return x - x0
}

// priceW: drawPrice's width, without drawing.
func priceW(c Cost, scale float64) int {
	w := 0.0
	if c[Money] > 0 {
		w += float64((len("$"+fmtNum(c[Money]))+1)*glyphW) * scale
	}
	if c[Ecto] > 0 {
		w += float64((len(skullRows[0])+2)+len(fmtNum(c[Ecto]))*glyphW) * scale
	}
	return int(w)
}

func (g *Game) drawTerminal(s *ebiten.Image) {
	rect(s, termX, termY, termW, termH, assets.Ink)
	rect(s, winX, winY, winW, termH-16, assets.Wall)

	// The address bar: three window dots, the URL, the cart badge.
	rect(s, winX, winY, winW, barH, assets.Floor)
	for k, c := range []color.Color{assets.Locked, assets.Money, assets.Accent} {
		rect(s, winX+12+k*14, winY+barH/2-4, 8, 8, c)
	}
	rect(s, winX+60, winY+6, 420, barH-12, assets.Ink)
	g.tintedAt(s, "hauntedhotel.shop/"+strings.ToLower(tabNames[g.termTab]), winX+70, winY+10, 1, assets.Ghost, 1)
	units := 0
	for _, l := range g.cart {
		units += l.n
	}
	g.tintedAt(s, fmt.Sprintf("CART (%d)", units), winX+winW-150, winY+2, 2, assets.Ghost, 1)

	// The tabs.
	tw := winW / int(tabs)
	for t := range tabs {
		x := winX + int(t)*tw
		if t == g.termTab {
			rect(s, x+4, tabY, tw-8, tabH, assets.Floor)
			rect(s, x+4, tabY+tabH-4, tw-8, 4, assets.Accent)
		}
		name := tabNames[t]
		if t == tabCart {
			name = fmt.Sprintf("CART %d", units)
		}
		g.tintedAt(s, name, x+tw/2-len(name)*glyphW, tabY+2, 2, assets.Ghost, 1)
	}

	switch {
	case g.placing >= 0:
		g.drawPlacing(s)
	case g.termTab == tabCart:
		g.drawCart(s)
	default:
		g.drawCatalog(s)
	}

	help := "A/D TAB   W/S ITEM   SPACE ADD TO CART   ESC CLOSE"
	switch {
	case g.placing >= 0 && shop[g.placing].ID == rowChange:
		help = "A/D ROOM   W/S BEDS   SPACE ADD TO CART   ESC BACK"
	case g.placing >= 0:
		help = "A/D ROOM   SPACE ADD TO CART   ESC BACK"
	case g.termTab == tabCart:
		help = "A/D TAB   W/S LINE   SPACE REMOVE ONE / PLACE ORDER / CANCEL A CHANGE   ESC CLOSE"
	}
	g.tintedAt(s, help, winX+12, termY+termH-30, 1, assets.Accent, 1)
	g.tintedAt(s, clockText(g.clock), winX+winW-12-glyphW*len(clockText(g.clock)), termY+termH-30, 1, assets.Ghost, 1)
}

// drawRow is one list row: highlight, icon, name and, at the right, a price or a word.
func (g *Game) drawRow(s *ebiten.Image, k int, it Item, name string, c Cost, word string, a float32) {
	g.drawRowAt(s, k, k == g.termPick, it, name, c, word, a)
}

// drawRowAt: drawRow on list row k, picked or not.
func (g *Game) drawRowAt(s *ebiten.Image, k int, picked bool, it Item, name string, c Cost, word string, a float32) {
	y := bodyY + k*rowH
	if picked {
		rect(s, listX, y, listW, rowH-4, assets.Floor)
		rect(s, listX, y, 4, rowH-4, assets.Accent)
	}
	g.drawIcon(s, it, listX+24, y+(rowH-4)/2, 32, a)
	g.tintedAt(s, name, listX+46, y+2, 2, assets.Ghost, a)
	if word != "" {
		g.tintedAt(s, word, listX+listW-8-len(word)*glyphW*2, y+2, 2, assets.Floor, 1)
		return
	}
	g.drawPrice(s, c, listX+listW-8-priceW(c, 2), y+2, 2)
}

func (g *Game) drawCatalog(s *ebiten.Image) {
	rows := tabItems(g.termTab)
	for k, i := range rows {
		it, a, word := shop[i], float32(1), ""
		switch {
		case !g.onSale(i):
			a, word = 0.35, "SOON"
		case it.unlock() && g.unlocked(it.ID):
			a, word = 0.6, "OWNED"
		case !g.afford(g.priceOf(i)):
			a = 0.6
		}
		g.drawRow(s, k, it, strings.ToUpper(it.Name), g.priceOf(i), word, a)
	}

	// The detail pane: the picked item, big.
	i := rows[g.termPick]
	it := shop[i]
	rect(s, paneX, bodyY, paneW, paneH, assets.Floor)
	a := float32(1)
	if !g.onSale(i) {
		a = 0.35
	}
	g.drawIcon(s, it, paneX+paneW/2, bodyY+56, 96, a)
	y := bodyY + 112
	g.tintedAt(s, strings.ToUpper(it.Name), paneX+12, y, 2, assets.Ghost, 1)
	g.tintedAt(s, it.Blurb, paneX+12, y+32, 1, assets.Ghost, 1)
	g.drawPrice(s, g.priceOf(i), paneX+12, y+54, 2)
	var status []string
	switch {
	case !g.onSale(i):
		status = append(status, "COMING SOON")
		for _, u := range it.Needs {
			if !g.unlocked(u) {
				status = append(status, "NEEDS "+strings.ToUpper(shop[slices.IndexFunc(shop, func(x Item) bool { return x.ID == u })].Name))
			}
		}
	case it.Tab == tabSupplies:
		status = append(status, fmt.Sprintf("%d PER CASE   IN STOCK: %d", it.Pack, g.stock[it.ID]))
	case it.unlock() && g.unlocked(it.ID):
		status = append(status, "OWNED")
	case it.ID == rowChange:
		status = append(status, "PICK THE ROOM, THEN ITS BEDS", "IT CHANGES WHEN THE ROOM IS EMPTY")
	case it.room():
		status = append(status, "DELIVERED TO A ROOM OF YOUR CHOICE")
	}
	for k, v := range status {
		g.tintedAt(s, v, paneX+12, y+92+k*18, 1, assets.Accent, 1)
	}
	switch {
	case !g.onSale(i) || it.unlock() && g.owned(i):
	case g.affords(i):
		g.drawButton(s, "ADD TO CART", paneX+12, bodyY+paneH-44, paneW-24)
	default:
		g.tintedAt(s, "NOT ENOUGH IN THE WALLET", paneX+12, bodyY+paneH-34, 2, assets.Locked, 1)
	}
}

// drawButton: a web button, label centered.
func (g *Game) drawButton(s *ebiten.Image, label string, x, y, w int) {
	rect(s, x, y, w, 34, assets.Accent)
	g.tintedAt(s, label, x+w/2-len(label)*glyphW, y+1, 2, assets.Ink, 1)
}

func (g *Game) drawCart(s *ebiten.Image) {
	for k, l := range g.cart {
		it := shop[l.item]
		name := strings.ToUpper(it.Name)
		switch {
		case l.beds != "":
			name = g.hotel.Rooms[l.room].Number + " > " + strings.ToUpper(l.beds)
		case l.room >= 0:
			name += " > " + g.hotel.Rooms[l.room].Number
		case l.n > 1:
			name = fmt.Sprintf("%dx %s", l.n, name)
		}
		g.drawRow(s, k, it, name, l.cost(), "", 1)
	}
	if len(g.cart) == 0 {
		g.tintedAt(s, "YOUR CART IS EMPTY", listX+12, bodyY+8, 2, assets.Floor, 1)
	}
	if w := g.pending(); len(w) > 0 { // the CHANGES list, under the cart's lines: SPACE cancels one, refunded
		at := max(len(g.cart), 1) // a list row for the heading, then one each
		g.tintedAt(s, "CHANGES WAITING FOR THEIR ROOMS (SPACE CANCELS, REFUNDED)", listX+12, bodyY+at*rowH+10, 1, assets.Accent, 1)
		for k, c := range w {
			it := shop[slices.IndexFunc(shop, func(it Item) bool { return it.ID == rowChange })]
			name := g.hotel.Rooms[c.room].Number + " > " + strings.ToUpper(c.to.Beds)
			g.drawRowAt(s, at+1+k, g.termPick == len(g.cart)+1+k, it, name, c.paid, "", 1)
		}
	}
	rect(s, paneX, bodyY, paneW, paneH, assets.Floor)
	g.tintedAt(s, "ORDER TOTAL", paneX+12, bodyY+12, 2, assets.Ghost, 1)
	g.drawPrice(s, g.cartTotal(), paneX+12, bodyY+52, 3)
	if len(g.cart) > 0 && !g.afford(g.cartTotal()) {
		g.tintedAt(s, "NOT ENOUGH IN THE WALLET", paneX+12, bodyY+110, 1, assets.Locked, 1)
	}
	g.tintedAt(s, "DELIVERED TO THE HOTEL", paneX+12, bodyY+130, 1, assets.Accent, 1)
	y := bodyY + paneH - 44
	if g.termPick == len(g.cart) {
		rect(s, paneX+8, y-4, paneW-16, 42, assets.Ghost)
	}
	g.drawButton(s, "PLACE ORDER", paneX+12, y, paneW-24)
}

// drawPlacing asks a Service's Room: the Room cards, the picked one outlined.
func (g *Game) drawPlacing(s *ebiten.Image) {
	it := shop[g.placing]
	if it.ID == rowChange { // the bed layout too, and what it costs
		// the layouts to pick from, each with its fee, the picked one marked as a tab is
		g.tintedAt(s, "W/S BEDS", listX, bodyY, 2, assets.Accent, 1)
		x := listX + 9*glyphW*2
		for k, b := range bedKinds {
			if !g.bedOpen(k) {
				continue
			}
			name, c, a := strings.ToUpper(b.Beds), g.bedFee(k), float32(0.5)
			w := (len(name)+1)*glyphW*2 + priceW(c, 2) + 16
			if k == g.termBeds {
				a = 1
				rect(s, x, bodyY-2, w, 38, assets.Floor)
				rect(s, x, bodyY+32, w, 4, assets.Accent)
			}
			g.tintedAt(s, name, x+8, bodyY, 2, assets.Ghost, a)
			g.drawPrice(s, c, x+8+(len(name)+1)*glyphW*2, bodyY, 2)
			x += w + 4
		}
	} else {
		g.tintedAt(s, "DELIVER "+strings.ToUpper(it.Name)+" TO WHICH ROOM?", listX, bodyY, 2, assets.Ghost, 1)
	}
	cols, rows := g.hotel.Grid()
	gx, gy, gw, gh := listX, bodyY+44, winW-32, paneH-44
	tw, th := (gw-(cols-1)*tileGap)/cols, (gh-(rows-1)*tileGap)/rows
	for i, room := range g.hotel.Rooms {
		x, y := gx+i%cols*(tw+tileGap), gy+i/cols*(th+tileGap)
		if i == g.termCursor {
			rect(s, x-3, y-3, tw+6, th+6, assets.Accent)
		}
		bg := assets.Floor
		if !g.vacant(i, Human) {
			bg = assets.Ink // grayed: Booked
		}
		rect(s, x, y, tw, th, bg)
		title := room.Number + " " + strings.ToUpper(room.Type.Beds)
		for _, c := range g.changes {
			if c.room == i && c.to.Beds != room.Type.Beds {
				title += " > " + strings.ToUpper(c.to.Beds)
			}
		}
		g.tintedAt(s, title, x+8, y+4, 2, assets.Ghost, 1)
		if !room.Mirror.Empty() {
			g.drawIcon(s, Item{Icon: "icon_mirror"}, x+tw-16, y+th-20, 32, 1)
		}
		if !g.isOwned(i) {
			g.tintedAt(s, "BOARDED UP", x+8, y+44, 2, assets.Locked, 1)
		}
		var heads []string
		for _, p := range g.parties {
			if p.Occupies(i, Human) {
				for _, guest := range p.Guests {
					heads = append(heads, guest.Character)
				}
			}
		}
		g.drawHeads(s, x+8, y+44, heads...)
		sx := x + 8
		if sh := g.shades[i]; len(sh) > 0 {
			g.drawHead(s, sx, y+66, sh[0].name, 0)
			sx += head.Dx() + 6
		}
		for _, pr := range g.priests {
			if pr.RoomIndex == i && !pr.done {
				g.drawHeads(s, sx, y+66, pr.Character)
				text(s, strings.ToUpper(pr.Character), sx+head.Dx()+4, y+66, assets.Accent)
			}
		}
		g.tintedAt(s, fmt.Sprintf("MOOD %d", g.roomMood(i)), x+8, y+th-22, 1, resInk(Ecto), 1)
	}
}
