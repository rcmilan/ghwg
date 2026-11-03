package game

import (
	"fmt"
	"image"
	"strings"

	"ghosthotel/assets"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
)

// openTerminal pauses the world on the Terminal (D73); with nobody at the counter, OK is selected.
func (g *Game) openTerminal() {
	g.state, g.message, g.terminalPick = StateTerminal, "", -1
	g.terminalOK = !g.atCounter()
}

// updateTerminal (D97, D98): WASD moves across the Room grid, and S from its bottom row reaches the OK
// button. SPACE on a Room picks it for the front Party, and picking another moves the pick. SPACE on OK
// books the pick, which is final, and closes the Terminal. ESC closes it without booking.
func (g *Game) updateTerminal() {
	if g.terminalOK {
		switch {
		case inpututil.IsKeyJustPressed(ebiten.KeyW):
			g.terminalOK = false
		case inpututil.IsKeyJustPressed(ebiten.KeySpace):
			if g.terminalPick >= 0 {
				g.book(g.terminalPick)
			}
			g.state, g.message = StatePlay, ""
		}
		return
	}
	cols, rows := g.hotel.Grid()
	c, r := g.terminalCursor%cols, g.terminalCursor/cols
	switch {
	case inpututil.IsKeyJustPressed(ebiten.KeyA):
		c = (c + cols - 1) % cols
	case inpututil.IsKeyJustPressed(ebiten.KeyD):
		c = (c + 1) % cols
	case inpututil.IsKeyJustPressed(ebiten.KeyW):
		r = max(r-1, 0)
	case inpututil.IsKeyJustPressed(ebiten.KeyS):
		g.terminalOK = r == rows-1
		r = min(r+1, rows-1)
	case inpututil.IsKeyJustPressed(ebiten.KeySpace):
		if g.message = g.fits(g.terminalCursor); g.message == "" {
			g.terminalPick = g.terminalCursor
			g.message = "Picked room " + g.hotel.Rooms[g.terminalCursor].Number + ". OK to confirm."
		}
	}
	g.terminalCursor = r*cols + c
}

// Terminal layout, screen px.
const (
	termX, termY, termW, termH = 120, 50, 720, 420
	gridX, gridY, gridW, gridH = 140, 100, 440, 330 // the Room tiles
	cardX, cardY               = 610, 100           // the front Party's card
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
	text(s, "RECEPTION TERMINAL", termX+24, termY+18, assets.Ghost)
	text(s, clockText(g.clock), termX+termW-24-6*len(clockText(g.clock)), termY+18, assets.Ghost)
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
		text(s, room.Number+" "+room.Type.Name, x+6, y+4, assets.Ghost)
		text(s, fmt.Sprintf("$%d/night", room.Type.Rate), x+6, y+20, assets.Ghost)
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
				text(s, fmt.Sprintf("CHECKOUT DAY %d", day(p.Checkout)), x+6, y+56, assets.Ghost)
			}
		}
		g.drawHeads(s, x+6, y+40, heads...)
		text(s, strings.ToUpper(room.Type.Beds), x+46, y+38, assets.Ghost) // Twin vs Double as text (D97)
	}

	text(s, fmt.Sprintf("QUEUE %d/%d", len(g.queue), len(g.hotel.Queue)), cardX, cardY, assets.Ghost)
	if !g.atCounter() {
		text(s, "no guests at the counter", cardX, cardY+32, assets.Ghost)
	} else {
		p := g.queue[0]
		var names []string
		for _, guest := range p.Guests {
			names = append(names, guest.Character)
		}
		text(s, "NEXT GUEST", cardX, cardY+32, assets.Ghost)
		g.drawHeads(s, cardX, cardY+52, names...)
		text(s, fmt.Sprintf("NIGHTS  %d", p.Nights), cardX, cardY+74, assets.Ghost)
		text(s, "ROUTINE "+p.Routine.String(), cardX, cardY+90, assets.Ghost)
		wish := "ANY BED"
		if p.Wish != "" {
			wish = strings.ToUpper(p.Wish)
		}
		text(s, "WISH    "+wish, cardX, cardY+106, assets.Ghost)
		i := g.terminalCursor
		if g.terminalPick >= 0 {
			i = g.terminalPick
		}
		room := g.hotel.Rooms[i]
		payout := "-"
		if g.fits(i) == "" {
			payout = fmt.Sprintf("$%d", p.rateIn(room)*p.Nights)
			if p.rateIn(room) < room.Type.Rate {
				payout += " (-50%: not " + strings.ToUpper(p.Wish) + ")"
			}
		}
		text(s, "ROOM    "+room.Number, cardX, cardY+122, assets.Ghost)
		text(s, "PAYOUT  "+payout, cardX, cardY+138, assets.Ghost)
	}
	text(s, "WASD SELECT   SPACE PICK   OK CONFIRM", termX+24, termY+termH-30, assets.Accent)
	const okW, okH = 80, 28
	okX, okY := termX+termW-24-okW, termY+termH-16-okH
	if g.terminalOK {
		rect(s, okX-3, okY-3, okW+6, okH+6, assets.Accent)
	}
	rect(s, okX, okY, okW, okH, assets.Floor)
	text(s, "OK", okX+okW/2-6, okY+okH/2-8, assets.Ghost)
}
