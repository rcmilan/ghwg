package game

import (
	"fmt"
	"image"
	"strings"

	"ghosthotel/assets"

	"github.com/hajimehoshi/ebiten/v2"
)

// openTerminal pauses the world on the Terminal (D73) on its Room list. ESC closes it.
func (g *Game) openTerminal() { g.state, g.message, g.termRoom = StateTerminal, "", -1 }

// The Room menu's picks, top to bottom.
const (
	pickWeak = iota
	pickStrong
	pickShadow
	picks
)

// updateTerminal (D151): WASD moves across the Room grid and SPACE opens a Room's menu; there, W/S picks
// Father K (weak), Father M (strong, D166) or a Shadow Person (D169), and SPACE pays, back to the list
// with what happened.
func (g *Game) updateTerminal() {
	if g.termRoom >= 0 {
		n := g.hotel.Rooms[g.termRoom].Number
		switch {
		case g.in.JustPressed(ebiten.KeyW):
			g.termPick = (g.termPick + picks - 1) % picks
		case g.in.JustPressed(ebiten.KeyS):
			g.termPick = (g.termPick + 1) % picks
		case g.in.JustPressed(ebiten.KeySpace) && g.termPick == pickShadow:
			if g.message = g.buyShadow(g.termRoom); g.message == "" {
				g.message = "A SHADOW LURKS IN ROOM " + n
			}
			g.termRoom = -1
		case g.in.JustPressed(ebiten.KeySpace):
			if g.message = g.callPriest(g.termRoom, g.termPick == pickStrong); g.message == "" {
				g.message = strings.ToUpper(g.priests[len(g.priests)-1].Character) + " CALLED TO ROOM " + n
			}
			g.termRoom = -1
		}
		return
	}
	cols, rows := g.hotel.Grid()
	c, r := g.termCursor%cols, g.termCursor/cols
	switch {
	case g.in.JustPressed(ebiten.KeyA):
		c = max(c-1, 0)
	case g.in.JustPressed(ebiten.KeyD):
		c = min(c+1, cols-1)
	case g.in.JustPressed(ebiten.KeyW):
		r = max(r-1, 0)
	case g.in.JustPressed(ebiten.KeyS):
		r = min(r+1, rows-1)
	case g.in.JustPressed(ebiten.KeySpace):
		g.termRoom, g.termPick, g.message = g.termCursor, pickWeak, ""
	}
	g.termCursor = r*cols + c
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
	cur := "$" + fmtNum(g.wallet[Money]) + "  SPOOK " + fmtNum(g.wallet[Spook]) // ghost shows only its own currency (D152); real both (D169)
	if g.ghost {
		cur = "SPOOK " + fmtNum(g.wallet[Spook])
	}
	text(s, cur, termX+termW/2, termY+18, assets.Money)

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
		if i == g.termRoom {
			for k, o := range [picks]string{fmt.Sprintf("FATHER K $%d: HALVES SPOOK", priestWeak),
				fmt.Sprintf("FATHER M $%d: CLEARS IT, BANISHES A SHADOW", priestStrong), fmt.Sprintf("SHADOW PERSON: %d SPOOK", shadowPrice)} {
				if k == g.termPick {
					text(s, ">", x+6, y+92+k*18, assets.Ghost)
				}
				g.drawHead(s, x+16, y+92+k*18, [picks]string{fatherK.name, fatherM.name, "Adam"}[k], [picks]float32{1, 1, 0}[k])
				text(s, o, x+16+head.Dx()+4, y+92+k*18, assets.Ghost)
			}
		}
	}

	help := "WASD SELECT   SPACE PRIESTS AND GHOSTS   ESC CLOSE"
	if g.termRoom >= 0 {
		help = "W/S PICK   SPACE PAY   ESC CLOSE"
	}
	text(s, help, termX+24, termY+termH-30, assets.Accent)
}
