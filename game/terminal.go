package game

import (
	"fmt"
	"image"
	"strings"

	"ghosthotel/assets"

	"github.com/hajimehoshi/ebiten/v2"
)

// openTerminal pauses the world on the Terminal (D73). It is read-only (T086): SPACE or ESC closes it.
func (g *Game) openTerminal() { g.state, g.message = StateTerminal, "" }

func (g *Game) updateTerminal() {
	if g.in.JustPressed(ebiten.KeySpace) {
		g.state, g.message = StatePlay, ""
	}
}

// Terminal layout, screen px.
const (
	termX, termY, termW, termH = 120, 50, 720, 420
	gridX, gridY, gridW, gridH = 140, 100, 680, 330 // the Room tiles
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
	cur := fmt.Sprintf("$%d", g.money) // each dimension shows its own currency (D152)
	if g.ghost {
		cur = fmt.Sprintf("SPOOK %d", g.spook)
	}
	text(s, cur, termX+termW/2, termY+18, assets.Money)

	cols, rows := g.hotel.Grid()
	tw, th := (gridW-(cols-1)*tileGap)/cols, (gridH-(rows-1)*tileGap)/rows
	for i, room := range g.hotel.Rooms {
		x, y := gridX+i%cols*(tw+tileGap), gridY+i/cols*(th+tileGap)
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
	}

	text(s, "SPACE CLOSE", termX+24, termY+termH-30, assets.Accent)
}
