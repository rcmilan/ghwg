package game

import (
	"fmt"
	"image"
	"image/color"
	"slices"
	"strings"

	"ghosthotel/entities"
	"ghosthotel/world"

	"github.com/hajimehoshi/ebiten/v2"
)

// The guest card (08 T375, G2): SPACE on the Room lock plate by a door shows who is in the Room (a ghost's, from
// its door in the ghost dimension): name, age and job (a ghost's death year and cause) and its line. REVEAL, 10
// Ecto and once a Party, adds Nerve (Phantasm) and the quirk, and for a named ghost its fact; a named guest stays
// revealed on later visits (revealedKeys, saved). MOVE (a human Party) is the talk (talkmove.go). It reuses the
// talk's state, camera and slow motion (g.parley.card set); A/D flips a Party of two, W/S picks, ESC leaves.
const (
	revealEcto = 10   // placeholder
	cardWrap   = 34   // characters a line of the card wraps at
	cardSlow   = 0.25 // of the world's time scale while a card is open
)

// card is the open card: which guest of the Party, and which of its options.
type card struct{ who, pick int }

type cardOpt int

const (
	optReveal cardOpt = iota
	optMove
)

// cardParty: the Party the plate of Room i shows: its human Party, or in the ghost dimension its ghost one.
func (g *Game) cardParty(i int) *Party {
	realm := Human
	if g.ghost {
		realm = Ghost
	}
	for _, p := range g.parties {
		if p.Occupies(i, realm) {
			return p
		}
	}
	return nil
}

// revealed: p's guest has been read: its Party was, or (a named guest) an earlier visit's was.
func (g *Game) revealed(p *Party, guest *entities.Guest) bool {
	return p.revealed || guest.Who.Key != "" && g.revealedKeys[guest.Who.Key]
}

// canMove: the human Party could be talked to about moving (the talk's conditions, minus where the Concierge stands).
func (g *Game) canMove(p *Party) bool {
	return p.Realm == Human && g.inRoom(p) && !p.abed && g.clock >= p.snubbed && len(g.targets(p)) > 0
}

// cardOpts: the options that apply to the guest now, in order. REVEAL stays while unaffordable (drawn grey).
func (g *Game) cardOpts(p *Party, guest *entities.Guest) (os []cardOpt) {
	if !g.revealed(p, guest) {
		os = append(os, optReveal)
	}
	if g.canMove(p) {
		os = append(os, optMove)
	}
	return os
}

// reveal pays for the REVEAL of the Party: both of a pair, and every named one for good. False if it can't be paid.
func (g *Game) reveal(p *Party) bool {
	if p.revealed || !g.pay(Cost{Ecto: revealEcto}) {
		return false
	}
	p.revealed = true
	for _, guest := range p.Guests {
		if k := guest.Who.Key; k != "" {
			if g.revealedKeys == nil {
				g.revealedKeys = map[string]bool{}
			}
			g.revealedKeys[k] = true
		}
	}
	return true
}

// startCard opens the card of Room i's Party, closed in on the plate with the world slowed.
func (g *Game) startCard(i int) {
	p := g.cardParty(i)
	if p == nil {
		return
	}
	g.state, g.parley = StateTalk, &parley{p: p, card: &card{}, scale: g.scale}
	g.scale *= cardSlow
}

// updateCard reads the card's keys; it closes by itself when its Party leaves.
func (g *Game) updateCard() {
	t := g.parley
	c, p := t.card, t.p
	if !slices.Contains(g.parties, p) || !p.Occupies(p.Room, p.Realm) {
		g.endTalk()
		return
	}
	key, n := g.in.JustPressed, len(p.Guests)
	switch {
	case key(ebiten.KeyA):
		c.who = (c.who + n - 1) % n
	case key(ebiten.KeyD):
		c.who = (c.who + 1) % n
	}
	os := g.cardOpts(p, p.Guests[c.who])
	if len(os) == 0 {
		c.pick = 0
	} else {
		switch {
		case key(ebiten.KeyW):
			c.pick += len(os) - 1
		case key(ebiten.KeyS):
			c.pick++
		}
		c.pick %= len(os)
	}
	if !key(ebiten.KeySpace) {
		return
	}
	switch {
	case len(os) == 0:
		g.endTalk()
	case os[c.pick] == optReveal:
		g.reveal(p)
	default:
		g.endTalk()
		g.startTalk(p)
	}
}

// cardRow is one line of the card.
type cardRow struct {
	text string
	ink  color.Color
	a    float32
}

func wrap(s string) (out []string) {
	line := ""
	for _, w := range strings.Fields(s) {
		if line != "" && len(line)+1+len(w) > cardWrap {
			out, line = append(out, line), ""
		}
		line = strings.TrimSpace(line + " " + w)
	}
	return append(out, line)
}

// cardRows: what the card shows for the picked guest, then its options, the picked one in the Ectoplasm ink.
func (g *Game) cardRows() []cardRow {
	p, c := g.parley.p, g.parley.card
	guest := p.Guests[c.who]
	w, white := guest.Who, color.Color(color.White)
	rows := []cardRow{{w.Name, resInk(Ecto), 1}}
	ghost := p.Realm == Ghost
	about := fmt.Sprintf("AGE %d - %s", w.Age, w.Job)
	switch {
	case ghost:
		about = fmt.Sprintf("DIED %d - %s", w.Died, w.Cause)
	case w.Age == 0:
		about = fmt.Sprintf("AGE %s - %s", strings.ToUpper(w.AgeNote), w.Job)
	}
	for _, l := range wrap(about) {
		rows = append(rows, cardRow{l, white, 1})
	}
	for _, l := range wrap("\"" + w.Card + "\"") {
		rows = append(rows, cardRow{l, white, 0.8})
	}
	if g.revealed(p, guest) {
		stat := fmt.Sprintf("NERVE %d", nerve(guest))
		if ghost {
			stat = fmt.Sprintf("PHANTASM %d", guest.Phantasm)
		}
		q := "NO QUIRK"
		if w.Quirk != NoQuirk {
			q = "QUIRK " + strings.ToUpper(quirkName(w.Quirk))
		}
		rows = append(rows, cardRow{stat, white, 1}, cardRow{q, white, 1})
		if f := ghostFact(w.Key); ghost && f != "" {
			for _, l := range wrap(f) {
				rows = append(rows, cardRow{l, white, 0.8})
			}
		}
	}
	can := g.afford(Cost{Ecto: revealEcto})
	for k, o := range g.cardOpts(p, guest) {
		text, ink, a := "MOVE", white, float32(0.6)
		if o == optReveal {
			text = fmt.Sprintf("REVEAL  %d ECTO", revealEcto)
			if !can { // grey, as the Terminal greys what the wallet can't pay
				ink, a = color.Gray{120}, 0.5
			}
		}
		if k == c.pick {
			if o == optMove || can {
				ink = resInk(Ecto)
			}
			a = 1
		}
		rows = append(rows, cardRow{text, ink, a})
	}
	if len(p.Guests) > 1 {
		rows = append(rows, cardRow{fmt.Sprintf("A/D  GUEST %d OF %d", c.who+1, len(p.Guests)), white, 0.6})
	}
	return rows
}

// ghostFact: a named ghost's fact, "" for any other.
func ghostFact(key string) string {
	for _, n := range namedGhosts {
		if key != "" && n.Key == key {
			return n.Fact
		}
	}
	return ""
}

// cardFocus: where the card's camera closes in (drawParley's screen px at the play camera) and how wide the sharp
// middle is: on the plate.
func (g *Game) cardFocus(ox, oy int, k float64) (fx, fy, sharp float64) {
	pl := g.hotel.Rooms[g.parley.p.Room].Plate()
	c := pl.Min.Add(pl.Max).Div(2)
	return float64(screenPx(float64(c.X)) - ox), float64(screenPx(float64(c.Y)) - oy), 3 * world.TileSize * zoom * k
}

// drawCard draws the card's panel over the closed-in world.
func (g *Game) drawCard(s *ebiten.Image, w, h int) {
	const scale, pad = 2, 16
	rows, lh := g.cardRows(), glyphH*2+6
	pw, ph := cardWrap*glyphW*scale+2*pad, len(rows)*lh+2*pad
	x0, y0 := w/2-pw/2, h/2-ph/2
	rect(s, x0, y0, pw, ph, color.RGBA{10, 6, 18, 225})
	for i, r := range rows {
		outline(s, r.text, x0+pad, y0+pad+i*lh, scale, r.ink, r.a)
	}
}

// DevCard (-card), the prints' scenes over -match's two Rooms: 0 a human's card, 1 revealed, 2 a Party of two,
// 3 its ghost's card, revealed.
func (g *Game) DevCard(n int) {
	if n < 0 {
		return
	}
	g.DevMatch(0)
	p, lead := g.humanIn(0), g.humanIn(0).Guests[0]
	if n == 2 {
		mate := entities.NewGuest(image.Pt(int(lead.X), int(lead.Y)), g.dressed("Adam"))
		mate.Leader, mate.FleeAt, mate.RoomIndex = lead, lead.FleeAt, 0
		g.rollHuman(&mate)
		p.Guests, p.due, p.cents = append(p.Guests, &mate), nil, nil
	}
	pl := g.hotel.Rooms[0].Plate()
	g.player.X, g.player.Y = float64(pl.Min.X+world.TileSize/2), float64(pl.Min.Y+world.TileSize/2)
	if n == 3 {
		g.ghost = true
	}
	g.startCard(0)
	if n == 1 || n == 3 {
		g.wallet[Ecto] = max(g.wallet[Ecto], revealEcto)
		g.reveal(g.parley.p)
	}
}

// revealedList: the revealed named guests' keys, in order, for the save.
func (g *Game) revealedList() []string {
	ks := make([]string, 0, len(g.revealedKeys))
	for k := range g.revealedKeys {
		ks = append(ks, k)
	}
	slices.Sort(ks)
	return ks
}
