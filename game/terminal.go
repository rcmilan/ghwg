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
	g.state, g.termTab = StateTerminal, min(g.termTab, tabs-1)
	g.refuse("")
	g.snapPick()
	g.see()
}

// see marks the fresh items of the open tab seen this Run: the Terminal's "!" no longer counts them (hud.go).
func (g *Game) see() {
	if g.seen == nil {
		g.seen = map[Unlock]bool{}
	}
	for _, i := range tabItems(g.termTab) {
		if g.fresh(i) {
			g.seen[shop[i].ID] = true
		}
	}
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

// gone: a store item that can't be bought any more this Run: an unlock bought or on its way, the hotel or the
// staff at their max. The store doesn't list it (08 F0-T: menus show only what applies; a teaser stays).
func (g *Game) gone(i int) bool {
	it := shop[i]
	return it.unlock() && (g.unlocked(it.ID) || g.coming(it.ID, -1)) ||
		it.Grow != 0 && strings.HasPrefix(g.growRefuse(it.Grow), "THE HOTEL") || it.Hire && g.staffed() >= keeperMax || it.ID == rowRetain && g.fatherIn(false)
}

// listed: the rows tab t shows, as indexes into tabItems(t), which g.termPick also is.
func (g *Game) listed(t Tab) []int {
	var l []int
	for k, i := range tabItems(t) {
		if !g.gone(i) {
			l = append(l, k)
		}
	}
	return l
}

// snapPick keeps the pick on a row there is: the cart's checkout at most, else the tab's first listed row.
func (g *Game) snapPick() {
	if g.termTab == tabCart {
		g.termPick = min(g.termPick, len(g.cart))
	} else if l := g.listed(g.termTab); len(l) > 0 && !slices.Contains(l, g.termPick) {
		g.termPick = l[0]
	}
}

// pickStep moves the pick d rows (W -1, S +1), wrapping: the cart's lines and checkout, or the tab's listed rows.
func (g *Game) pickStep(d int) {
	l := g.listed(g.termTab)
	if g.termTab == tabCart {
		l = make([]int, len(g.cart)+1)
		for k := range l {
			l[k] = k
		}
	}
	if k := slices.Index(l, g.termPick); k >= 0 {
		g.termPick = l[(k+d+len(l))%len(l)]
	}
}

// scrollTop: the first of n list rows shown, rows at a time, so that row pos shows, centered when it can be.
func scrollTop(pos, n, rows int) int { return max(0, min(pos-rows/2, n-rows)) }

// updateTerminal (D180): A/D picks the tab, W/S the row, SPACE adds to the cart, removes a cart line or
// checks out.
func (g *Game) updateTerminal() {
	key := g.in.JustPressed
	said := "" // a move drops the notice; SPACE says how it went
	switch {
	case key(ebiten.KeyA):
		g.termTab, g.termPick = (g.termTab+tabs-1)%tabs, 0
		g.snapPick()
		g.see()
	case key(ebiten.KeyD):
		g.termTab, g.termPick = (g.termTab+1)%tabs, 0
		g.snapPick()
		g.see()
	case key(ebiten.KeyW):
		g.pickStep(-1)
	case key(ebiten.KeyS):
		g.pickStep(1)
	case key(ebiten.KeySpace):
		said = g.press()
	default:
		return
	}
	g.refuse(said)
}

// press is SPACE on the picked row: what the store says back.
func (g *Game) press() string {
	if g.termTab == tabCart {
		if g.termPick < len(g.cart) {
			g.removeLine(g.termPick)
			g.termPick = min(g.termPick, len(g.cart))
			return ""
		}
		if why := g.placeOrder(); why != "" {
			return why
		}
		g.termPick, g.state = 0, StatePlay // an order placed closes the store
		return ""
	}
	if !slices.Contains(g.listed(g.termTab), g.termPick) {
		return "NOTHING TO BUY HERE"
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
		if it.Tab == tabHotel || it.Once && it.Tab == tabStaff {
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

// Terminal layout, screen px: a browser window over the whole screen (08 F0-T).
const (
	barH               = 44                         // the address bar, with the wallet and its rates
	tabY, tabH         = barH + 8, 36               // the tabs
	bodyY              = tabY + tabH + 10           // the list and the detail pane
	footY              = screenH - 26               // the help line
	listX, listW, rowH = 16, 352, 38                // the item list
	listRows           = (footY - 8 - bodyY) / rowH // rows in sight; more scroll
	paneX              = listX + listW + 20
	paneW, paneH       = screenW - 16 - paneX, listRows*rowH - 4
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
	"icon_deluxe":     {{"prop_bed_brown_r0c0", "prop_bed_brown_r0c1"}, {"prop_bed_brown_r1c0", "prop_bed_brown_r1c1"}, {"prop_bed_brown_r2c0", "prop_bed_brown_r2c1"}},
	"icon_grow_col":   {{"door_plain_top", "door_plain_top"}, {"door_plain_bottom", "door_plain_bottom"}},
	"icon_grow_floor": {{"prop_stairs_r0c0", "prop_stairs_r0c1"}, {"prop_stairs_r1c0", "prop_stairs_r1c1"}},
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

// drawPrice draws "$N" in the store's Money ink and a skull with N in its Ecto ink, from (x,y), all in red when the
// wallet can't pay it; it returns the width.
func (g *Game) drawPrice(s *ebiten.Image, c Cost, x, y int, scale float64, red bool) int {
	if g.skull == nil {
		g.skull = bitmap(skullRows)
	}
	money, ecto := color.Color(storeMoney), color.Color(storeEcto)
	if red {
		money, ecto = assets.Locked, assets.Locked
	}
	x0 := x
	if c[Money] > 0 {
		v := "$" + fmtNum(c[Money])
		g.tintedAt(s, v, x, y, scale, money, 1)
		x += int(float64(len(v)*glyphW+glyphW) * scale)
	}
	if c[Ecto] > 0 {
		g.stamp(s, g.skull, x, y+int(skullDY*scale), scale, ecto)
		x += int(float64(len(skullRows[0])+2) * scale)
		v := fmtNum(c[Ecto])
		g.tintedAt(s, v, x, y, scale, ecto, 1)
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
	rect(s, 0, 0, screenW, screenH, storePage)

	// The address bar: three window dots, the URL, the wallet and its rates.
	rect(s, 0, 0, screenW, barH, storeBar)
	for k, c := range []color.Color{assets.Locked, assets.Money, storeAccent} {
		rect(s, 16+k*14, barH/2-4, 8, 8, c)
	}
	rect(s, 64, 8, 300, barH-16, storeInk)
	g.tintedAt(s, "hauntedhotel.shop/"+strings.ToLower(tabNames[g.termTab]), 74, barH/2-8, 1, storeBarInk, 1)
	g.drawRates(s)

	// The tabs.
	units := 0
	for _, l := range g.cart {
		units += l.n
	}
	tw := screenW / int(tabs)
	for t := range tabs {
		x := int(t) * tw
		if t == g.termTab {
			rect(s, x+8, tabY, tw-16, tabH, storeCard)
			rect(s, x+8, tabY+tabH-4, tw-16, 4, storeAccent)
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
	g.tintedAt(s, help, listX, footY, 1, storeAccent, 1)
	g.tintedAt(s, clockText(g.clock), screenW-16-glyphW*len(clockText(g.clock)), footY, 1, storeInk, 1)
}

// drawRates: the wallet and what it earns a real minute, at the bar's right (08 F0-T: the rates show only here).
func (g *Game) drawRates(s *ebiten.Image) {
	if g.skull == nil {
		g.skull = bitmap(skullRows)
	}
	rate, x := g.perMinute(), screenW-16
	for _, r := range []Res{Ecto, Money} { // right to left
		v, per := fmtNum(g.wallet[r]), "+"+fmtNum(rate[r])+"/MIN"
		if r == Money {
			v, per = "$"+v, "+$"+fmtNum(rate[r])+"/MIN"
		}
		x -= len(per) * glyphW
		g.tintedAt(s, per, x, barH/2-8, 1, storeBarInk, 1)
		x -= 8 + len(v)*glyphW*2
		g.tintedAt(s, v, x, barH/2-16, 2, resInk(r), 1)
		if r == Ecto {
			x -= (len(skullRows[0]) + 2) * 2
			g.stamp(s, g.skull, x, barH/2-16+skullDY*2, 2, resInk(r))
		}
		x -= 28
	}
}

// drawRow is one cart row: highlight, icon, name and, at the right, a price or a word.
func (g *Game) drawRow(s *ebiten.Image, k int, it Item, name string, c Cost, word string, a float32) {
	g.drawRowAt(s, k, k == g.termPick, it, name, c, word, a)
}

// drawRowAt: a row on list row k, picked or not, its price red when the wallet can't pay it.
func (g *Game) drawRowAt(s *ebiten.Image, k int, picked bool, it Item, name string, c Cost, word string, a float32) {
	g.drawLine(s, bodyY+k*rowH, picked, it, name, c, word, a, !g.afford(c))
}

// drawLine: a list row at y.
func (g *Game) drawLine(s *ebiten.Image, y int, picked bool, it Item, name string, c Cost, word string, a float32, red bool) {
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
	g.drawPrice(s, c, listX+listW-8-priceW(c, 2), y+2, 2, red)
}

// drawScroll: the list's scroll marker, when its n rows don't all show; top is the first in sight.
func drawScroll(s *ebiten.Image, top, n int) {
	if n <= listRows {
		return
	}
	x, h := listX+listW+6, paneH
	rect(s, x, bodyY, 4, h, storeCard)
	rect(s, x, bodyY+h*top/n, 4, h*listRows/n, storeAccent)
}

// about: the detail pane's line: what unlocks a teaser (its Hint), else its Blurb (08 F0-T).
func (g *Game) about(i int) string {
	if !g.onSale(i) {
		return shop[i].Hint
	}
	return shop[i].Blurb
}

func (g *Game) drawCatalog(s *ebiten.Image) {
	items, l := tabItems(g.termTab), g.listed(g.termTab)
	if len(l) == 0 {
		g.tintedAt(s, "NOTHING LEFT TO BUY", listX+12, bodyY+8, 2, storeDim, 1)
		return
	}
	pos := max(0, slices.Index(l, g.termPick))
	top := scrollTop(pos, len(l), listRows)
	for k := top; k < min(top+listRows, len(l)); k++ {
		i := items[l[k]]
		it, a, word := shop[i], float32(1), ""
		if !g.onSale(i) {
			a, word = 0.35, "SOON"
		}
		g.drawLine(s, bodyY+(k-top)*rowH, k == pos, it, strings.ToUpper(it.Name), g.priceOf(i), word, a, !g.affords(i))
	}
	drawScroll(s, top, len(l))

	// The detail pane: the picked item, big.
	i := items[l[pos]]
	it := shop[i]
	rect(s, paneX, bodyY, paneW, paneH, storeCard)
	a := float32(1)
	if !g.onSale(i) {
		a = 0.35
	}
	g.drawIcon(s, it, paneX+paneW/2, bodyY+76, 128, a)
	y := bodyY + 160
	g.tintedAt(s, strings.ToUpper(it.Name), paneX+16, y, 3, storeInk, 1)
	g.tintedAt(s, g.about(i), paneX+16, y+54, 1, storeInk, 1)
	g.drawPrice(s, g.priceOf(i), paneX+16, y+80, 2, g.onSale(i) && !g.affords(i))
	status := ""
	switch {
	case !g.onSale(i):
	case g.owned(i):
		status = "IN THE CART"
	case it.Tab == tabSupplies:
		status = fmt.Sprintf("%d PER CASE   IN STOCK: %d", it.Pack, g.stock[it.ID])
	case it.Hire:
		status = fmt.Sprintf("$%d A DAY   HIRED: %d", keeperWage, g.staffed())
	}
	g.tintedAt(s, status, paneX+16, y+120, 1, storeAccent, 1)
	switch {
	case !g.onSale(i) || g.owned(i):
	case g.affords(i):
		g.drawButton(s, "ADD TO CART", paneX+16, bodyY+paneH-50, paneW-32)
	default:
		g.tintedAt(s, "CAN'T AFFORD", paneX+16, bodyY+paneH-42, 2, assets.Locked, 1)
	}
}

// drawButton: a web button, label centered.
func (g *Game) drawButton(s *ebiten.Image, label string, x, y, w int) {
	rect(s, x, y, w, 34, storeAccent)
	g.tintedAt(s, label, x+w/2-len(label)*glyphW, y+1, 2, storeCard, 1)
}

func (g *Game) drawCart(s *ebiten.Image) {
	top := scrollTop(min(g.termPick, len(g.cart)-1), len(g.cart), listRows)
	for k := top; k < min(top+listRows, len(g.cart)); k++ {
		l := g.cart[k]
		it := shop[l.item]
		name := strings.ToUpper(it.Name)
		if l.n > 1 {
			name = fmt.Sprintf("%dx %s", l.n, name)
		}
		g.drawRowAt(s, k-top, k == g.termPick, it, name, l.cost(), "", 1)
	}
	drawScroll(s, top, len(g.cart))
	if len(g.cart) == 0 {
		g.tintedAt(s, "YOUR CART IS EMPTY", listX+12, bodyY+8, 2, storeDim, 1)
	}
	rect(s, paneX, bodyY, paneW, paneH, storeCard)
	g.tintedAt(s, "ORDER TOTAL", paneX+16, bodyY+16, 2, storeInk, 1)
	g.drawPrice(s, g.cartTotal(), paneX+16, bodyY+56, 3, !g.afford(g.cartTotal()))
	if len(g.cart) > 0 && !g.afford(g.cartTotal()) {
		g.tintedAt(s, "CAN'T AFFORD", paneX+16, bodyY+114, 1, assets.Locked, 1)
	}
	g.tintedAt(s, "DELIVERED TO THE HOTEL", paneX+16, bodyY+134, 1, storeAccent, 1)
	y := bodyY + paneH - 50
	if g.termPick == len(g.cart) {
		rect(s, paneX+12, y-4, paneW-24, 42, storeInk)
	}
	g.drawButton(s, "PLACE ORDER", paneX+16, y, paneW-32)
}
