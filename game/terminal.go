package game

import (
	"fmt"
	"image"
	"image/color"
	"slices"
	"strings"

	"shift/assets"

	"github.com/hajimehoshi/ebiten/v2"
)

// openTerminal pauses the world on the Terminal (D73); with nobody at the counter, OK is selected.
func (g *Game) openTerminal() {
	g.state, g.message, g.terminalPick, g.terminalParty, g.terminalList = StateTerminal, "", -1, nil, false
	g.terminalOK = len(g.counter()) == 0
}

// counter: the Parties standing at the counter, in arrival order (04 D118, D119).
func (g *Game) counter() []*Party {
	var ps []*Party
	for _, p := range g.queue {
		if p.standing() {
			ps = append(ps, p)
		}
	}
	return ps
}

// fitting: the Parties at the counter the Room fits, in arrival order.
func (g *Game) fitting(room int) []*Party {
	var ps []*Party
	for _, p := range g.counter() {
		if g.fitsParty(p, room) == "" {
			ps = append(ps, p)
		}
	}
	return ps
}

// updateTerminal (D97, D98, 04 D119): WASD moves across the Room grid, and S from its bottom row reaches
// the OK button. SPACE on a Room picks it, then the Party: the first one it fits is chosen, and with more
// than one the list at the right takes the focus (W/S, SPACE chooses, A goes back to the grid); OK books
// the pick, which is final, and closes the Terminal. ESC closes it without booking.
func (g *Game) updateTerminal() {
	switch {
	case g.terminalOK:
		switch {
		case g.in.JustPressed(ebiten.KeyW):
			g.terminalOK = false
		case g.in.JustPressed(ebiten.KeySpace):
			if g.terminalParty != nil {
				g.bookParty(g.terminalParty, g.terminalPick, false)
			}
			g.state, g.message = StatePlay, ""
		}
		return
	case g.terminalList:
		fit := g.fitting(g.terminalPick)
		i := slices.Index(fit, g.terminalParty)
		switch {
		case g.in.JustPressed(ebiten.KeyW):
			i = max(i-1, 0)
		case g.in.JustPressed(ebiten.KeyS):
			i = min(i+1, len(fit)-1)
		case g.in.JustPressed(ebiten.KeyA):
			g.terminalList = false
		case g.in.JustPressed(ebiten.KeySpace):
			g.terminalList, g.terminalOK = false, true
			g.message = "Picked room " + g.hotel.Rooms[g.terminalPick].Number + ". OK to confirm."
		}
		g.terminalParty = fit[i]
		return
	}
	cols, rows := g.hotel.Grid()
	c, r := g.terminalCursor%cols, g.terminalCursor/cols
	switch {
	case g.in.JustPressed(ebiten.KeyA):
		c = (c + cols - 1) % cols
	case g.in.JustPressed(ebiten.KeyD):
		c = (c + 1) % cols
	case g.in.JustPressed(ebiten.KeyW):
		r = max(r-1, 0)
	case g.in.JustPressed(ebiten.KeyS):
		g.terminalOK = r == rows-1
		r = min(r+1, rows-1)
	case g.in.JustPressed(ebiten.KeySpace):
		g.pickRoom(g.terminalCursor)
	}
	g.terminalCursor = r*cols + c
}

// pickRoom picks the Room and the first Party at the counter it fits: straight to OK if it fits only
// that one, else into the list. Nobody fits: refused, with why (for the front Party).
func (g *Game) pickRoom(room int) {
	fit := g.fitting(room)
	if len(fit) == 0 {
		g.message = "No guests at the counter."
		if ps := g.counter(); len(ps) > 0 {
			g.message = g.fitsParty(ps[0], room)
		}
		return
	}
	g.terminalPick, g.terminalParty = room, fit[0]
	g.message = "Picked room " + g.hotel.Rooms[room].Number + ". OK to confirm."
	if g.terminalOK = len(fit) == 1; !g.terminalOK {
		g.terminalList, g.message = true, "Picked room "+g.hotel.Rooms[room].Number+". Choose the guests."
	}
}

// Terminal layout, screen px.
const (
	termX, termY, termW, termH = 120, 50, 720, 420
	gridX, gridY, gridW, gridH = 140, 100, 440, 330 // the Room tiles
	cardX, cardY, cardW, rowH  = 610, 100, 206, 62  // the Parties at the counter, one row each
	tileGap                    = 8
)

// head is a character's head on the idle sheet, facing down: the Terminal's size icon (D97, T010).
var head = image.Rect(18*charW, 9, 19*charW, 22)

// drawHeads draws one shrunk head per character, left to right from (x,y).
func (g *Game) drawHeads(s *ebiten.Image, x, y int, names ...string) {
	for i, name := range names {
		op := &ebiten.DrawImageOptions{}
		op.GeoM.Translate(float64(x+i*(head.Dx()+2)), float64(y))
		s.DrawImage(g.characters[name][0].SubImage(head).(*ebiten.Image), op)
	}
}

func (g *Game) drawTerminal(s *ebiten.Image) {
	rect(s, termX, termY, termW, termH, assets.Ink)
	rect(s, termX+8, termY+8, termW-16, termH-16, assets.Wall)
	text(s, "RECEPTION TERMINAL", termX+24, termY+18, assets.Pale)
	text(s, clockText(g.clock), termX+termW-24-6*len(clockText(g.clock)), termY+18, assets.Pale)
	text(s, fmt.Sprintf("$%d", g.money), termX+termW/2, termY+18, assets.Money)

	cols, rows := g.hotel.Grid()
	tw, th := (gridW-(cols-1)*tileGap)/cols, (gridH-(rows-1)*tileGap)/rows
	for i, room := range g.hotel.Rooms {
		x, y := gridX+i%cols*(tw+tileGap), gridY+i/cols*(th+tileGap)
		if i == g.terminalCursor && !g.terminalOK {
			rect(s, x-3, y-3, tw+6, th+6, assets.Accent)
		}
		bg := assets.Floor
		if !g.vacant(i) {
			bg = assets.Ink // grayed: Booked
		}
		rect(s, x, y, tw, th, bg)
		rect(s, x+tw-12, y+4, 8, 8, panelColor(g.autoRoom[i])) // its Door Panel, read-only (04 D114)
		text(s, room.Number+" "+room.Type.Name, x+6, y+4, assets.Pale)
		text(s, fmt.Sprintf("$%d/night", room.Type.Rate), x+6, y+20, assets.Pale)
		heads := make([]string, len(room.Stays)) // Vacant: the capacity, as Adam's heads
		for k := range heads {
			heads[k] = guestCast[0]
		}
		if i == g.terminalPick {
			text(s, "PICKED", x+6, y+56, assets.Accent)
		}
		for _, p := range g.parties {
			if p.Occupies(i) { // Booked: its own Party's heads, side by side, only while it's in the Room (S2 review)
				heads = heads[:0]
				if lead := p.Guests[0]; p.state != out && image.Pt(int(lead.X), int(lead.Y)).In(room.Shell()) {
					for _, guest := range p.Guests {
						heads = append(heads, guest.Character)
					}
				}
				text(s, fmt.Sprintf("CHECKOUT DAY %d", day(p.Checkout)), x+6, y+56, assets.Pale)
			}
		}
		g.drawHeads(s, x+6, y+40, heads...)
		text(s, strings.ToUpper(room.Type.Beds), x+46, y+38, assets.Pale) // Twin vs Double as text (D97)
	}

	text(s, fmt.Sprintf("QUEUE %d/%d", len(g.queue), len(g.hotel.Queue)), cardX, cardY, assets.Pale)
	ps := g.counter()
	if len(ps) == 0 {
		text(s, "no guests at the counter", cardX, cardY+32, assets.Pale)
	}
	room := g.terminalCursor // the payout line: the pick, else the Room under the cursor
	if g.terminalPick >= 0 {
		room = g.terminalPick
	}
	chosen := g.terminalParty
	for k, p := range ps { // one row per Party at the counter, in arrival order (04 D119)
		y := cardY + 24 + k*rowH
		if p == chosen {
			rect(s, cardX-6, y-4, cardW+12, rowH-4, assets.Accent)
			rect(s, cardX-3, y-1, cardW+6, rowH-10, assets.Wall)
		}
		var names []string
		for _, guest := range p.Guests {
			names = append(names, guest.Character)
		}
		g.drawHeads(s, cardX, y, names...)
		text(s, fmt.Sprintf("%d NIGHT%s  %s", p.Nights, map[bool]string{true: "S"}[p.Nights > 1], p.Routine), cardX+40, y, assets.Pale)
		wish := "ANY BED"
		if p.Wish != "" {
			wish = strings.ToUpper(p.Wish)
		}
		text(s, "WISH "+wish, cardX+40, y+16, assets.Pale)
		if g.terminalPick >= 0 && g.fitsParty(p, room) != "" { // greyed: the picked Room can't take it
			rect(s, cardX-3, y-1, cardW+6, rowH-10, color.RGBA{18, 22, 35, 190})
		}
	}
	if chosen == nil && len(ps) > 0 {
		chosen = ps[0]
	}
	if chosen != nil {
		r := g.hotel.Rooms[room]
		payout := "-"
		if g.fitsParty(chosen, room) == "" {
			payout = fmt.Sprintf("$%d", chosen.rateIn(r)*chosen.Nights)
			if chosen.rateIn(r) < r.Type.Rate {
				payout += " (-50%: not " + strings.ToUpper(chosen.Wish) + ")"
			}
		}
		text(s, "ROOM    "+r.Number, cardX, cardY+24+3*rowH, assets.Pale)
		text(s, "PAYOUT  "+payout, cardX, cardY+40+3*rowH, assets.Pale)
	}
	text(s, "WASD SELECT   SPACE PICK ROOM, THEN GUESTS   OK CONFIRM", termX+24, termY+termH-30, assets.Accent)
	const okW, okH = 80, 28
	okX, okY := termX+termW-24-okW, termY+termH-16-okH
	if g.terminalOK {
		rect(s, okX-3, okY-3, okW+6, okH+6, assets.Accent)
	}
	rect(s, okX, okY, okW, okH, assets.Floor)
	text(s, "OK", okX+okW/2-6, okY+okH/2-8, assets.Pale)
}
