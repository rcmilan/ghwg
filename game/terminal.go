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
	g.state, g.message, g.termTab = StateTerminal, "", min(g.termTab, tabs-1)
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

// rowCount: rows in the open tab; the cart's lines, then the checkout.
func (g *Game) rowCount() int {
	if g.termTab == tabCart {
		return len(g.cart) + 1
	}
	return len(tabItems(g.termTab))
}

// updateTerminal (D180): A/D picks the tab, W/S the row, SPACE adds to the cart, removes a cart line or
// checks out.
func (g *Game) updateTerminal() {
	key := g.in.JustPressed
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
		g.termPick, g.state = 0, StatePlay // an order placed closes the store
		return ""
	}
	i := tabItems(g.termTab)[g.termPick]
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

// The store's colors (07 U1, the "haunted shop" of the 2026-11-14 grill): a lavender page, white cards, a
// purple browser bar, an ecto-green accent. The hotel view's cards and the pause use them too.
var (
	storePage   = color.RGBA{0xe9, 0xe4, 0xf2, 255}
	storeCard   = color.RGBA{0xf8, 0xf6, 0xfc, 255}
	storeInk    = color.RGBA{0x2a, 0x21, 0x40, 255}
	storeDim    = color.RGBA{0xa3, 0x9c, 0xb8, 255}
	storeBar    = color.RGBA{0x4a, 0x3a, 0x78, 255}
	storeBarInk = color.RGBA{0xf2, 0xed, 0xff, 255}
	storeAccent = color.RGBA{0x2a, 0xa0, 0x7a, 255}
	storeMoney  = color.RGBA{0x7a, 0x3f, 0xb0, 255}
	storeEcto   = color.RGBA{0x1f, 0x8f, 0x6a, 255}
)

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

// drawPrice draws "$N" in the store's Money ink and a skull with N in its Ecto ink, from (x,y); it returns the width.
func (g *Game) drawPrice(s *ebiten.Image, c Cost, x, y int, scale float64) int {
	if g.skull == nil {
		g.skull = bitmap(skullRows)
	}
	x0 := x
	if c[Money] > 0 {
		v := "$" + fmtNum(c[Money])
		g.tintedAt(s, v, x, y, scale, storeMoney, 1)
		x += int(float64(len(v)*glyphW+glyphW) * scale)
	}
	if c[Ecto] > 0 {
		g.stamp(s, g.skull, x, y+int(skullDY*scale), scale, storeEcto)
		x += int(float64(len(skullRows[0])+2) * scale)
		v := fmtNum(c[Ecto])
		g.tintedAt(s, v, x, y, scale, storeEcto, 1)
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
	rect(s, termX, termY, termW, termH, storeInk)
	rect(s, winX, winY, winW, termH-16, storePage)

	// The address bar: three window dots, the URL, the cart badge.
	rect(s, winX, winY, winW, barH, storeBar)
	for k, c := range []color.Color{assets.Locked, assets.Money, storeAccent} {
		rect(s, winX+12+k*14, winY+barH/2-4, 8, 8, c)
	}
	rect(s, winX+60, winY+6, 420, barH-12, storeInk)
	g.tintedAt(s, "hauntedhotel.shop/"+strings.ToLower(tabNames[g.termTab]), winX+70, winY+10, 1, storeBarInk, 1)
	units := 0
	for _, l := range g.cart {
		units += l.n
	}
	g.tintedAt(s, fmt.Sprintf("CART (%d)", units), winX+winW-150, winY+2, 2, storeBarInk, 1)

	// The tabs.
	tw := winW / int(tabs)
	for t := range tabs {
		x := winX + int(t)*tw
		if t == g.termTab {
			rect(s, x+4, tabY, tw-8, tabH, storeCard)
			rect(s, x+4, tabY+tabH-4, tw-8, 4, storeAccent)
		}
		name := tabNames[t]
		if t == tabCart {
			name = fmt.Sprintf("CART %d", units)
		}
		g.tintedAt(s, name, x+tw/2-len(name)*glyphW, tabY+2, 2, storeInk, 1)
	}

	help := "A/D TAB   W/S ITEM   SPACE ADD TO CART   ESC CLOSE"
	if g.termTab == tabCart {
		g.drawCart(s)
		help = "A/D TAB   W/S LINE   SPACE REMOVE ONE / PLACE ORDER   ESC CLOSE"
	} else {
		g.drawCatalog(s)
	}
	g.tintedAt(s, help, winX+12, termY+termH-30, 1, storeAccent, 1)
	g.tintedAt(s, clockText(g.clock), winX+winW-12-glyphW*len(clockText(g.clock)), termY+termH-30, 1, storeInk, 1)
}

// drawRow is one list row: highlight, icon, name and, at the right, a price or a word.
func (g *Game) drawRow(s *ebiten.Image, k int, it Item, name string, c Cost, word string, a float32) {
	g.drawRowAt(s, k, k == g.termPick, it, name, c, word, a)
}

// drawRowAt: drawRow on list row k, picked or not.
func (g *Game) drawRowAt(s *ebiten.Image, k int, picked bool, it Item, name string, c Cost, word string, a float32) {
	y := bodyY + k*rowH
	if picked {
		rect(s, listX, y, listW, rowH-4, storeCard)
		rect(s, listX, y, 4, rowH-4, storeAccent)
	}
	g.drawIcon(s, it, listX+24, y+(rowH-4)/2, 32, a)
	g.tintedAt(s, name, listX+46, y+2, 2, storeInk, a)
	if word != "" {
		g.tintedAt(s, word, listX+listW-8-len(word)*glyphW*2, y+2, 2, storeDim, 1)
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
	rect(s, paneX, bodyY, paneW, paneH, storeCard)
	a := float32(1)
	if !g.onSale(i) {
		a = 0.35
	}
	g.drawIcon(s, it, paneX+paneW/2, bodyY+56, 96, a)
	y := bodyY + 112
	g.tintedAt(s, strings.ToUpper(it.Name), paneX+12, y, 2, storeInk, 1)
	g.tintedAt(s, it.Blurb, paneX+12, y+32, 1, storeInk, 1)
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
	}
	for k, v := range status {
		g.tintedAt(s, v, paneX+12, y+92+k*18, 1, storeAccent, 1)
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
	rect(s, x, y, w, 34, storeAccent)
	g.tintedAt(s, label, x+w/2-len(label)*glyphW, y+1, 2, storeCard, 1)
}

func (g *Game) drawCart(s *ebiten.Image) {
	for k, l := range g.cart {
		it := shop[l.item]
		name := strings.ToUpper(it.Name)
		if l.n > 1 {
			name = fmt.Sprintf("%dx %s", l.n, name)
		}
		g.drawRow(s, k, it, name, l.cost(), "", 1)
	}
	if len(g.cart) == 0 {
		g.tintedAt(s, "YOUR CART IS EMPTY", listX+12, bodyY+8, 2, storeDim, 1)
	}
	rect(s, paneX, bodyY, paneW, paneH, storeCard)
	g.tintedAt(s, "ORDER TOTAL", paneX+12, bodyY+12, 2, storeInk, 1)
	g.drawPrice(s, g.cartTotal(), paneX+12, bodyY+52, 3)
	if len(g.cart) > 0 && !g.afford(g.cartTotal()) {
		g.tintedAt(s, "NOT ENOUGH IN THE WALLET", paneX+12, bodyY+110, 1, assets.Locked, 1)
	}
	g.tintedAt(s, "DELIVERED TO THE HOTEL", paneX+12, bodyY+130, 1, storeAccent, 1)
	y := bodyY + paneH - 44
	if g.termPick == len(g.cart) {
		rect(s, paneX+8, y-4, paneW-16, 42, storeInk)
	}
	g.drawButton(s, "PLACE ORDER", paneX+12, y, paneW-24)
}
