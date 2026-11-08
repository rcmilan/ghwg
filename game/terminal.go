package game

import (
	"fmt"
	"image"
	"image/color"
	"slices"
	"strings"

	"ghosthotel/assets"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
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

// rowCount: rows in the open tab; the cart's last row is the checkout.
func (g *Game) rowCount() int {
	if g.termTab == tabCart {
		return len(g.cart) + 1
	}
	return len(tabItems(g.termTab))
}

// updateTerminal (D180): A/D picks the tab, W/S the row, SPACE adds to the cart, removes a cart line or
// checks out. A Service first asks for its Room: A/D picks it, SPACE adds, ESC backs out (game.go).
func (g *Game) updateTerminal() {
	key := g.in.JustPressed
	if g.placing >= 0 {
		n := len(g.hotel.Rooms)
		switch {
		case key(ebiten.KeyA):
			g.termCursor, g.message = (g.termCursor+n-1)%n, ""
		case key(ebiten.KeyD):
			g.termCursor, g.message = (g.termCursor+1)%n, ""
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
		if why := g.placeOrder(); why != "" {
			return why
		}
		g.termPick = 0
		return "ORDER PLACED"
	}
	i := tabItems(g.termTab)[g.termPick]
	if shop[i].Tab == tabServices && g.onSale(i) {
		g.placing = i
		return ""
	}
	if why := g.addToCart(i, -1); why != "" {
		return why
	}
	return strings.ToUpper(shop[i].Name) + " IN THE CART"
}

// ShopPrint opens the store holding money and spook, every Hotel unlock still to buy: main's -shop dev
// start (T187).
func (g *Game) ShopPrint(money, spook float64) {
	g.locked = map[Unlock]bool{}
	for _, it := range shop {
		if it.Tab == tabHotel {
			g.locked[it.ID] = true
		}
	}
	g.wallet = [2]Num{Num(money), Num(spook)}
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

// icon: an item's picture, a cut sprite (A1) or a character's head; a Shadow's head is black.
func (g *Game) icon(it Item) (*ebiten.Image, float32) {
	if it.Head {
		c := float32(1)
		if it.ID == rowShadow {
			c = 0
		}
		return g.sheet(it.Icon)[0].SubImage(head).(*ebiten.Image), c
	}
	if g.icons == nil {
		g.icons = map[string]*ebiten.Image{}
	}
	img, ok := g.icons[it.Icon]
	if !ok {
		img, _, _ = ebitenutil.NewImageFromFileSystem(assets.FS, "sprites/"+it.Icon+".png") // nil if missing: drawn as nothing
		g.icons[it.Icon] = img
	}
	return img, 1
}

// drawIcon fits an item's icon in a box px square centered at (cx,cy), at alpha a.
func (g *Game) drawIcon(s *ebiten.Image, it Item, cx, cy, box int, a float32) {
	img, c := g.icon(it)
	if img == nil {
		return
	}
	w, h := img.Bounds().Dx(), img.Bounds().Dy()
	k := float64(box / max(w, h)) // whole pixels
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Scale(k, k)
	op.GeoM.Translate(float64(cx)-float64(w)*k/2, float64(cy)-float64(h)*k/2)
	op.ColorScale.Scale(c*a, c*a, c*a, a)
	s.DrawImage(img, op)
}

// drawPrice draws "$N" in Money's ink and a skull with N in Spook's, from (x,y); it returns the width.
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
	if c[Spook] > 0 {
		g.stamp(s, g.skull, x, y+int(skullDY*scale), scale, resInk(Spook))
		x += int(float64(len(skullRows[0])+2) * scale)
		v := fmtNum(c[Spook])
		g.tintedAt(s, v, x, y, scale, resInk(Spook), 1)
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
	if c[Spook] > 0 {
		w += float64((len(skullRows[0])+2)+len(fmtNum(c[Spook]))*glyphW) * scale
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
	case g.placing >= 0:
		help = "A/D ROOM   SPACE ADD TO CART   ESC BACK"
	case g.termTab == tabCart:
		help = "A/D TAB   W/S LINE   SPACE REMOVE ONE / PLACE ORDER   ESC CLOSE"
	}
	g.tintedAt(s, help, winX+12, termY+termH-30, 1, assets.Accent, 1)
	g.tintedAt(s, clockText(g.clock), winX+winW-12-glyphW*len(clockText(g.clock)), termY+termH-30, 1, assets.Ghost, 1)
}

// drawRow is one list row: highlight, icon, name and, at the right, a price or a word.
func (g *Game) drawRow(s *ebiten.Image, k int, it Item, name string, c Cost, word string, a float32) {
	y := bodyY + k*rowH
	if k == g.termPick {
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
		case it.Tab == tabHotel && g.unlocked(it.ID):
			a, word = 0.6, "OWNED"
		case !g.afford(it.Cost):
			a = 0.6
		}
		g.drawRow(s, k, it, strings.ToUpper(it.Name), it.Cost, word, a)
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
	g.drawPrice(s, it.Cost, paneX+12, y+54, 2)
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
	case it.Tab == tabHotel && g.unlocked(it.ID):
		status = append(status, "OWNED")
	case it.Tab == tabServices:
		status = append(status, "DELIVERED TO A ROOM OF YOUR CHOICE")
	}
	for k, v := range status {
		g.tintedAt(s, v, paneX+12, y+92+k*18, 1, assets.Accent, 1)
	}
	if g.onSale(i) && !(it.Tab == tabHotel && g.owned(i)) {
		g.drawButton(s, "ADD TO CART", paneX+12, bodyY+paneH-44, paneW-24)
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
	g.tintedAt(s, "DELIVER "+strings.ToUpper(it.Name)+" TO WHICH ROOM?", listX, bodyY, 2, assets.Ghost, 1)
	cols, rows := g.hotel.Grid()
	gx, gy, gw, gh := listX, bodyY+44, winW-32, paneH-44
	tw, th := (gw-(cols-1)*tileGap)/cols, (gh-(rows-1)*tileGap)/rows
	for i, room := range g.hotel.Rooms {
		x, y := gx+i%cols*(tw+tileGap), gy+i/cols*(th+tileGap)
		if i == g.termCursor {
			rect(s, x-3, y-3, tw+6, th+6, assets.Accent)
		}
		bg := assets.Floor
		if !g.vacant(i) {
			bg = assets.Ink // grayed: Booked
		}
		rect(s, x, y, tw, th, bg)
		g.tintedAt(s, room.Number+" "+strings.ToUpper(room.Type.Name), x+8, y+4, 2, assets.Ghost, 1)
		var heads []string
		for _, p := range g.parties {
			if p.Occupies(i) {
				for _, guest := range p.Guests {
					heads = append(heads, guest.Character)
				}
			}
		}
		g.drawHeads(s, x+8, y+44, heads...)
		sx := x + 8
		if sh := g.shades[i]; sh != nil {
			g.drawHead(s, sx, y+66, sh.name, 0)
			text(s, "SHADOW", sx+head.Dx()+4, y+66, assets.Accent)
			sx += head.Dx() + 4 + 6*len("SHADOW ")
		}
		for _, pr := range g.priests {
			if pr.RoomIndex == i && !pr.done {
				g.drawHeads(s, sx, y+66, pr.Character)
				text(s, strings.ToUpper(pr.Character), sx+head.Dx()+4, y+66, assets.Accent)
			}
		}
		g.tintedAt(s, fmt.Sprintf("SPOOK %d", g.roomSpook[i]), x+8, y+th-22, 1, resInk(Spook), 1)
	}
}
