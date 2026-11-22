package game

import (
	"fmt"
	"image"
	"image/color"
	"slices"
	"strconv"
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
	cardSlow   = 0.06 // of the world's time scale while a card is open: very slow
)

// card is the open card: which guest of the Party, which of its options, and which of the guest's lines it says.
type card struct {
	who, pick, line int
}

type cardOpt int

const (
	optReveal cardOpt = iota
	optMove
	optTell // 08 W5: the quest verbs, done by quest TALK (verbs_card.go), never on the Card
	optTake
	optGive
	optNoReveal // the MacGuffin: "REVEAL: IT REVEALS YOU", nothing to do
)

// cardParty: the Party the plate of Room i shows: its human Party, or in the ghost dimension its ghost one.
func (g *Game) cardParty(i int) *Party {
	realm := Human
	if g.ghost {
		realm = Ghost
	}
	for _, p := range g.parties {
		if p.Occupies(i, realm) || isMacguffin(p) && p.Occupies(i, Human) { // 08 W5: the MacGuffin's card is the same in both Dimensions
			return p
		}
	}
	return nil
}

// revealed: p's guest has been read: its Party was, or (a named guest) an earlier visit's was.
func (g *Game) revealed(p *Party, guest *entities.Guest) bool {
	return p.revealed || guest.Who.Key != "" && g.revealedKeys[guest.Who.Key] || isMacguffin(p) // 08 W5: nothing to reveal
}

// canMove: the human Party could be talked to about moving (the talk's conditions, minus where the Concierge stands).
func (g *Game) canMove(p *Party) bool {
	return p.Realm == Human && !isMacguffin(p) && g.inRoom(p) && !p.abed && g.clock >= p.snubbed && len(g.targets(p)) > 0
}

// cardOpts: the options that apply to the guest now, in order. REVEAL stays while unaffordable (drawn grey).
func (g *Game) cardOpts(p *Party, guest *entities.Guest) (os []cardOpt) {
	if _, priest := guestFather(p); guest.Who.Key == "macguffin" {
		os = append(os, optNoReveal)
	} else if !priest && !g.revealed(p, guest) { // a Father has nothing to reveal
		os = append(os, optReveal)
	}
	if g.canMove(p) {
		os = append(os, optMove)
	}
	return os
}

// revealCost: REVEAL's price: nothing with Show, don't tell (08 W5).
func (g *Game) revealCost() Num {
	if g.itemOn(itemBall) {
		return 0
	}
	return revealEcto
}

// reveal pays for the REVEAL of the Party: both of a pair, and every named one for good. False if it can't be paid.
func (g *Game) reveal(p *Party) bool {
	if p.revealed || !g.pay(Cost{Ecto: g.revealCost()}) {
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
	g.state, g.parley = StateTalk, &parley{p: p, card: &card{line: p.said}, scale: g.scale}
	p.said++ // the next opening says the guest's next line (T381 refine 5)
	g.verbOpened(p)
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

func wrap(s string) []string { return wrapAt(s, cardWrap) }

func wrapAt(s string, n int) (out []string) {
	line := ""
	for _, w := range strings.Fields(s) {
		if line != "" && len(line)+1+len(w) > n {
			out, line = append(out, line), ""
		}
		line = strings.TrimSpace(line + " " + w)
	}
	return append(out, line)
}

// cardView: what the card shows for the picked guest, by where it stands around the guest (drawCard): its line over
// its head, who it is on its left, what REVEAL read on its right, the options below.
type cardView struct {
	quote, who, read, opts []cardRow
}

// sideWrap: characters a line beside the guest wraps at.
const sideWrap = 20

func (g *Game) cardView() (v cardView) {
	p, c := g.parley.p, g.parley.card
	guest := p.Guests[c.who]
	w, white := guest.Who, color.Color(color.White)
	ghost := p.Realm == Ghost
	// "Name, age" over the job; a ghost's "Name, death year" over its cause (T381 refine 4: no labels, no dash).
	age, about := strconv.Itoa(w.Age), w.Job
	switch {
	case ghost:
		age, about = strconv.Itoa(w.Died), w.Cause
	case w.Age == 0:
		age = w.AgeNote
	}
	v.who = []cardRow{{w.Name + ", " + age, resInk(Ecto), 1}}
	for _, l := range wrapAt(about, sideWrap) {
		v.who = append(v.who, cardRow{l, white, 1})
	}
	said := ""
	if n := len(w.Lines); n > 0 {
		said = w.Lines[c.line%n]
	}
	if q := g.verbQuote(p, guest); q != "" { // 08 W5: a quest guest's Ask, or what it was just given
		said = q
	}
	if said != "" {
		for _, l := range wrap("\"" + said + "\"") {
			v.quote = append(v.quote, cardRow{l, white, 1})
		}
	}
	if isMacguffin(p) { // 08 W5: REVEAL: IT REVEALS YOU
		v.read = append(v.read, cardRow{macguffinCard.Reveal, white, 1})
	} else if g.revealed(p, guest) {
		stat := fmt.Sprintf("NERVE %d", nerve(guest))
		if ghost {
			stat = fmt.Sprintf("PHANTASM %d", guest.Phantasm)
		}
		q := "NO QUIRK"
		if w.Quirk != NoQuirk {
			q = "QUIRK " + strings.ToUpper(quirkName(w.Quirk))
		}
		rq, rf := g.verbRead(p, guest) // 08 W5: a quest guest's quirk and fact
		if rq != "" {
			q = "QUIRK " + rq
		}
		v.read = append(v.read, cardRow{stat, white, 1}, cardRow{q, white, 1})
		if f := ghostFact(w.Key); rf != "" || ghost && f != "" {
			if rf != "" {
				f = rf
			}
			for _, l := range wrapAt(f, sideWrap) {
				v.read = append(v.read, cardRow{l, white, 0.8})
			}
		}
	}
	can := g.afford(Cost{Ecto: g.revealCost()})
	for k, o := range g.cardOpts(p, guest) {
		text, ink, a := "MOVE", white, float32(0.6)
		if o == optNoReveal {
			text = macguffinCard.Reveal
		}
		if o == optReveal {
			text = "REVEAL"
			if c := g.revealCost(); c > 0 { // free with Show, don't tell: no price
				text = fmt.Sprintf("REVEAL  %d ECTO", int(c))
			}
			if !can { // grey, as the Terminal greys what the wallet can't pay
				ink, a = color.Gray{120}, 0.5
			}
		}
		if k == c.pick {
			if o != optReveal && o != optNoReveal || o == optReveal && can {
				ink = resInk(Ecto)
			}
			a = 1
		}
		v.opts = append(v.opts, cardRow{text, ink, a})
	}
	return v
}

// cardRows: the card's rows, in reading order.
func (g *Game) cardRows() []cardRow {
	v := g.cardView()
	return slices.Concat(v.who, v.quote, v.read, v.opts)
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

// drawCard draws the card around its guest in the talk's frame (drawParley): words alone, outlined; its line over
// its head, who it is on its left, what REVEAL read on its right, the options at the bottom over the cinema bar, and
// < > by them for a Party of two. at is the guest's feet on screen, top its head.
func (g *Game) drawCard(s *ebiten.Image, w, h, bar int, at, top image.Point) {
	const scale, gap = 2, 60 // gap: screen px from the guest's middle to the side columns, clear of its sprite
	v, lh := g.cardView(), glyphH*scale+8
	text := func(r cardRow, x, y int) { // kept on screen: by the map's edge the guest is off the middle
		outline(s, r.text, min(max(x, 8), w-8-len(r.text)*glyphW*scale), y, scale, r.ink, r.a)
	}
	mid := (at.Y+top.Y)/2 - lh/2
	for i, r := range v.quote {
		y := top.Y - 8 - (len(v.quote)-i)*lh
		text(r, at.X-len(r.text)*glyphW*scale/2, y)
	}
	y0 := mid // the columns hang from here, growing down: a long one never climbs over the line above the head
	for i, r := range v.who {
		text(r, at.X-gap-len(r.text)*glyphW*scale, y0+i*lh)
	}
	for i, r := range v.read {
		text(r, at.X+gap, y0+i*lh)
	}
	y0 = h - bar - 12 - max(1, len(v.opts))*lh
	for i, r := range v.opts {
		outline(s, r.text, w/2-len(r.text)*glyphW*scale/2, y0+i*lh, scale, r.ink, r.a)
	}
	if len(g.parley.p.Guests) > 1 { // the other guest of the pair: A and D
		y := y0 + (max(1, len(v.opts))-1)*lh/2
		outline(s, "<", w/2-300, y, scale, color.White, 0.8)
		outline(s, ">", w/2+300-glyphW*scale, y, scale, color.White, 0.8)
	}
}

// DevCard (-card), the prints' scenes over -match's two Rooms: 0 a human's card, 1 revealed, 2 a Party of two,
// 3 its ghost's card, revealed, 4 Father K as a guest.
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
	if n == 4 {
		g.becomeFather(p, false)
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

// drawPlate draws Room i's guest plate (T381 refine 2): a small brass plaque on the wall west of its door, where TALK
// opens the card. In ghost it reads in the Ectoplasm ink.
func (g *Game) drawPlate(s *ebiten.Image, i, ox, oy int) {
	frame, face, ink := color.RGBA{0x4a, 0x32, 0x12, 255}, color.RGBA{0xd9, 0xb0, 0x4c, 255}, color.RGBA{0x7a, 0x55, 0x1c, 255}
	if g.ghost {
		frame, face, ink = color.RGBA{0x10, 0x40, 0x32, 255}, color.RGBA{0x5f, 0xe0, 0xb0, 255}, color.RGBA{0x1f, 0x80, 0x60, 255}
	}
	c := g.hotel.Rooms[i].Plate().Min.Sub(image.Pt(0, world.TileSize)) // the wall cell above the plate's mat cell
	x, y := (c.X+world.TileSize-14)*zoom-ox, (c.Y+3)*zoom-oy
	rect(s, x, y, 9*zoom, 7*zoom, frame)
	rect(s, x+zoom, y+zoom, 7*zoom, 5*zoom, face)
	rect(s, x+2*zoom, y+2*zoom, 5*zoom, zoom, ink) // its engraved lines
	rect(s, x+2*zoom, y+4*zoom, 3*zoom, zoom, ink)
}
