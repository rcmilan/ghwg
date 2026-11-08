package game

import (
	"fmt"
	"image"
	"image/color"
	"slices"
	"strings"

	"ghosthotel/assets"

	"github.com/hajimehoshi/ebiten/v2"
)

// openTerminal pauses the world on the Terminal (D73), on its shop. ESC closes it.
func (g *Game) openTerminal() {
	g.state, g.message = StateTerminal, ""
	g.reveal()
	g.pickRow(0)
}

// updateTerminal (D180): W/S picks a listed shop row, A/D the Room a PerRoom row is bought for, SPACE buys.
func (g *Game) updateTerminal() {
	n := len(g.hotel.Rooms)
	switch {
	case g.in.JustPressed(ebiten.KeyW):
		g.pickRow(-1)
	case g.in.JustPressed(ebiten.KeyS):
		g.pickRow(1)
	case g.in.JustPressed(ebiten.KeyA):
		g.termCursor = (g.termCursor + n - 1) % n
	case g.in.JustPressed(ebiten.KeyD):
		g.termCursor = (g.termCursor + 1) % n
	case g.in.JustPressed(ebiten.KeySpace) && slices.Contains(g.listed(), g.termPick):
		it := shop[g.termPick]
		if g.message = g.buy(g.termPick, g.termCursor); g.message == "" {
			name, _, _ := strings.Cut(strings.ToUpper(it.Name), ":")
			g.message = name + " UNLOCKED"
			if it.PerRoom {
				g.message = name + " FOR ROOM " + g.hotel.Rooms[g.termCursor].Number
			}
		}
	}
	g.reveal() // a bought unlock may meet another row's Needs, and leaves the list
	g.pickRow(0)
}

// pickRow moves the Terminal's pick d listed rows, wrapping; 0 keeps it on a listed row.
func (g *Game) pickRow(d int) {
	rows := g.listed()
	if len(rows) == 0 {
		return
	}
	k := max(slices.Index(rows, g.termPick), 0)
	g.termPick = rows[(k+d+len(rows))%len(rows)]
}

// ShopPrint opens the shop holding money and spook, every unlock but the Mirror still locked: main's
// -shop dev print (T187).
func (g *Game) ShopPrint(money, spook float64) {
	g.locked = map[Unlock]bool{}
	for _, it := range shop {
		if it.Buy == nil && it.ID != Mirror {
			g.locked[it.ID] = true
		}
	}
	g.wallet = [2]Num{Num(money), Num(spook)}
	g.openTerminal()
}

// Terminal layout, screen px: the Room grid on top, the shop below.
const (
	termX, termY, termW, termH = 120, 30, 720, 470
	gridX, gridY, gridW, gridH = 140, 80, 680, 130 // the Room tiles
	tileGap                    = 8
	listX, listY, rowH         = 144, 226, 16
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

func (g *Game) drawTerminal(s *ebiten.Image) {
	rect(s, termX, termY, termW, termH, assets.Ink)
	rect(s, termX+8, termY+8, termW-16, termH-16, assets.Wall)
	text(s, "RECEPTION TERMINAL", termX+24, termY+18, assets.Ghost)
	text(s, clockText(g.clock), termX+termW-24-6*len(clockText(g.clock)), termY+18, assets.Ghost)

	cols, rows := g.hotel.Grid()
	tw, th := (gridW-(cols-1)*tileGap)/cols, (gridH-(rows-1)*tileGap)/rows
	for i, room := range g.hotel.Rooms {
		x, y := gridX+i%cols*(tw+tileGap), gridY+i/cols*(th+tileGap)
		if i == g.termCursor {
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
		sx := x + 6
		if g.closed(i) {
			text(s, "CLOSED", sx, y+72, assets.Accent)
			sx += 6 * len("CLOSED ")
		}
		if sh := g.shades[i]; sh != nil {
			g.drawHead(s, sx, y+72, sh.name, 0)
			text(s, "SHADOW", sx+head.Dx()+4, y+72, assets.Accent)
			sx += head.Dx() + 4 + 6*len("SHADOW ")
		}
		for _, pr := range g.priests {
			if pr.RoomIndex == i && !pr.done { // a Priest's name follows his head, as the Guests' heads (S8 review)
				g.drawHeads(s, sx, y+72, pr.Character)
				text(s, strings.ToUpper(pr.Character), sx+head.Dx()+4, y+72, assets.Accent)
			}
		}
	}
	g.drawShop(s)
	text(s, "W/S PICK   A/D ROOM   SPACE BUY   ESC CLOSE", termX+24, termY+termH-24, assets.Accent)
}

// drawShop lists the revealed rows: a black head and the price until affordable, then the head and name.
func (g *Game) drawShop(s *ebiten.Image) {
	rows := g.listed()
	if len(rows) == 0 {
		text(s, "NOTHING FOR SALE YET", listX+16, listY, assets.Ghost)
	}
	for k, i := range rows {
		it, y := shop[i], listY+k*rowH
		lit := g.afford(it.Cost)
		if i == g.termPick {
			text(s, ">", listX, y, assets.Ghost)
			if it.PerRoom {
				text(s, "< ROOM "+g.hotel.Rooms[g.termCursor].Number+" >", listX+360, y, assets.Ghost)
			}
		}
		switch {
		case it.Head != "":
			var c float32 // a silhouette; a Shadow's head is black anyway (D169)
			if lit && it.ID != rowShadow {
				c = 1
			}
			g.drawHead(s, listX+12, y+1, it.Head, c)
		case !lit:
			rect(s, listX+12, y+1, head.Dx(), head.Dy(), color.Black) // ponytail: no sprite yet, A1's icons replace it
		}
		if lit {
			text(s, strings.ToUpper(it.Name), listX+32, y, assets.Ghost)
		}
		x := listX + 500
		for r, part := range [2]string{"$" + fmtNum(it.Cost[Money]), fmtNum(it.Cost[Spook]) + " SPOOK"} {
			if it.Cost[r] > 0 {
				text(s, part, x, y, resInk(Res(r)))
				x += 6 * (len(part) + 2)
			}
		}
	}
}
