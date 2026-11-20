package game

import (
	"hash/fnv"
	"image"
	"image/color"
	"slices"
	"strconv"
	"strings"

	"ghosthotel/entities"
	"ghosthotel/world"

	"github.com/hajimehoshi/ebiten/v2"
)

// 08 W4 (T377): the notebook on the entrance counter, and the named cast's arrivals (context/08-quests/cast.md).
// Every Party leaves a line in the book as it goes: how the stay went decides which. Ghost entries show only in ghost.

// bookOutcome: how a stay ended.
type bookOutcome int

const (
	outCozy     bookOutcome = iota // paid, the Room as good as it was
	outDread                       // paid, but the Room rotted or the Party stepped out for air
	outFled                        // ran, or walked out of a dirty Room
	outPassed                      // a ghost left on its own
	outBanished                    // Father M sent a ghost over
)

const (
	bookMax   = 50        // entries kept
	namedOdds = 6         // a Party's booking is a named guest's 1 time in namedOdds
	namedAway = dayFrames // a named guest must have been out of the hotel this long to come again
	dreadMood = rotMood   // a stay that ended at or below this Room Mood was a dread one
	scribble  = "~ ^ ~ ~ ^ ~"
	bookRows  = 4 // entries a spread shows at least: A/D stops with these in sight
)

// bookEntry is one line of the notebook (saved in runState).
type bookEntry struct {
	Name string
	Line string // empty: a scribble
	Day  int
	Out  bookOutcome
}

func (e bookEntry) ghost() bool { return e.Out >= outPassed }

func b2o(b bool, yes, no bookOutcome) bookOutcome {
	if b {
		return yes
	}
	return no
}

// stayOutcome: a Party that paid went away cozy, or in dread when the Room rotted or it stepped out for air.
func (g *Game) stayOutcome(p *Party) bookOutcome {
	return b2o(p.scares > 0 || g.roomMood(p.Room) <= dreadMood, outDread, outCozy)
}

// logStay writes the Party's entry as it leaves and notes when its named guest left.
func (g *Game) logStay(p *Party, out bookOutcome) {
	names := make([]string, len(p.Guests))
	for i, guest := range p.Guests {
		names[i] = guest.Who.Name
	}
	g.guestBook = append(g.guestBook, bookEntry{strings.Join(names, " & "), g.bookLine(p, out), day(g.clock), out})
	if len(g.guestBook) > bookMax {
		g.guestBook = slices.Delete(g.guestBook, 0, len(g.guestBook)-bookMax)
	}
	if k := p.Guests[0].Who.Key; k != "" {
		if g.leftAt == nil {
			g.leftAt = map[string]int{}
		}
		g.leftAt[k] = g.clock
	}
}

// pickBy: a line of s chosen by the guests' names and the day, no random draw: a draw from any of the Game's rngs
// would move the sim (talk.go reads g.words to decide who chats).
func pickBy(s []string, seed string) string {
	h := fnv.New32a()
	h.Write([]byte(seed))
	return s[int(h.Sum32())%len(s)]
}

// bookLine picks the line: a named guest's own, else one of the generated tables.
func (g *Game) bookLine(p *Party, out bookOutcome) string {
	if k := p.Guests[0].Who.Key; k != "" {
		if l := namedLine(k, p.visit, out); l != "" || out == outFled {
			return l
		}
	}
	seed := p.Guests[0].Who.Name + strconv.Itoa(day(g.clock)) + outTag[out]
	switch out {
	case outDread:
		return pickBy(notebookDread, seed)
	case outFled:
		return pickBy(notebookFled, seed)
	case outPassed:
		return pickBy(notebookPassed, seed)
	case outBanished:
		return pickBy(notebookBanished, seed)
	}
	return pickBy(notebookCozy, seed)
}

// namedLine: a named guest's line for visit n (1 on): the visit's own, rotating from the 4th; the dread and fled lines
// win when the stay went badly. A Skeptic has no fled line (it never flees), so one that walked out gets "": a scribble.
// A ghost has no dread or fled, and a banished one takes the generated line ("" here).
func namedLine(key string, n int, out bookOutcome) string {
	i := (max(n, 1) - 1) % 3
	if j := slices.IndexFunc(namedHumans, func(h namedHuman) bool { return h.Key == key }); j >= 0 {
		h := namedHumans[j]
		switch out {
		case outDread:
			return h.Dread
		case outFled:
			return h.Fled
		}
		return h.Visit[i]
	}
	if j := slices.IndexFunc(namedGhosts, func(h namedGhost) bool { return h.Key == key }); j >= 0 && out == outPassed {
		return namedGhosts[j].Visit[i]
	}
	return ""
}

// namedFree: the named guest is not in the hotel and has been away namedAway.
func (g *Game) namedFree(key string) bool {
	for _, p := range g.parties {
		for _, guest := range p.Guests {
			if guest.Who.Key == key {
				return false
			}
		}
	}
	t, ok := g.leftAt[key]
	return !ok || g.clock-t >= namedAway
}

// osvaldoDue: Osvaldo comes only once Magda has booked this Run, and trails her visits.
func (g *Game) osvaldoDue() bool { return g.met["magda"] && g.visits["osvaldo"] < g.visits["magda"] }

// namedPick: this booking is a named guest's 1 time in namedOdds, one of the eligible keys. g.ident only.
func (g *Game) namedPick(keys []string) (string, bool) {
	if g.ident.IntN(namedOdds) != 0 {
		return "", false
	}
	keys = slices.DeleteFunc(keys, func(k string) bool { return !g.namedFree(k) || k == "osvaldo" && !g.osvaldoDue() })
	if len(keys) == 0 {
		return "", false
	}
	return keys[g.ident.IntN(len(keys))], true
}

// visited counts the visit (kept across Runs, save.go) and gives it to the Party.
func (g *Game) visited(p *Party, key string) {
	if g.visits == nil {
		g.visits = map[string]int{}
	}
	if g.met == nil {
		g.met = map[string]bool{}
	}
	g.visits[key]++
	g.met[key] = true
	p.visit = g.visits[key]
}

// rollNamedHuman: a Party that just booked may be a named human's: its Guests take the entry's Members (a pair needs
// two). Identity only: g.ident, never g.rng.
func (g *Game) rollNamedHuman(p *Party) {
	var keys []string
	for _, h := range namedHumans {
		if len(h.Members) <= len(p.Guests) {
			keys = append(keys, h.Key)
		}
	}
	key, ok := g.namedPick(keys)
	if !ok {
		return
	}
	h := namedHumans[slices.IndexFunc(namedHumans, func(h namedHuman) bool { return h.Key == key })]
	for i, m := range h.Members {
		guest := p.Guests[i]
		guest.Who = entities.Identity{Name: m.Name, Age: m.Age, AgeNote: m.AgeNote, Job: h.Job, Lines: m.Lines, Quirk: h.Quirk, Key: h.Key}
		if _, seed, ok := strings.Cut(guest.Character, "#"); ok { // its own base character, the look seed kept
			guest.Character = m.Sprite + "#" + seed
		}
	}
	g.visited(p, key)
}

// rollNamedGhost: a ghost Party that just booked may be a named ghost's, in its first guest's place (g.ident only);
// nameGhost keeps it to its cast.md kind.
func (g *Game) rollNamedGhost(p *Party) {
	keys := make([]string, len(namedGhosts))
	for i, n := range namedGhosts {
		keys[i] = n.Key
	}
	key, ok := g.namedPick(keys)
	if !ok {
		return
	}
	n := namedGhosts[slices.IndexFunc(namedGhosts, func(n namedGhost) bool { return n.Key == key })]
	g.nameGhost(p.Guests[0], n)
	g.visited(p, key)
}

// nameGhost: the guest becomes n, its rolled Phantasm clamped into n's kind's band so it draws as cast.md says.
func (g *Game) nameGhost(guest *entities.Guest, n namedGhost) {
	guest.Who = entities.Identity{Name: n.Name, Died: n.Died, Job: n.Job, Lines: n.Lines, Quirk: n.Quirk, Key: n.Key}
	k := slices.IndexFunc(ghostKinds[:], func(c ghostKind) bool { return c.Name == n.Kind })
	lo, hi := 1, phantasmMax
	if k > 0 {
		lo = phantasmBands[k-1]
	}
	if k < len(phantasmBands) {
		hi = phantasmBands[k] - 1
	}
	guest.Phantasm = min(max(guest.Phantasm, lo), hi)
}

// The book on the counter: SPACE beside it opens the page (actionable.go).

// A Strategy playing (the sim, -play) never opens it: the Pilot taps SPACE the moment it leaves the Terminal's zone,
// which is where the book's zone starts, and that would move the sim (ponytail: teach the Pilot the book if a Strategy
// ever wants to read it).
func (g *Game) atBook() bool { return g.auto == nil && g.feet().In(g.hotel.Book) }

// bookRect: the counter cell the book lies on, world px.
func (g *Game) bookRect() image.Rectangle {
	b := g.hotel.Book
	return image.Rect(b.Min.X, b.Min.Y+2*world.TileSize, b.Max.X, b.Max.Y-world.TileSize)
}

func (g *Game) openBook() { g.state, g.bookTop = StateBook, 0 }

// shown: the entries the open page lists, newest first: a human's view hides the ghosts'.
func (g *Game) shown() []bookEntry {
	var es []bookEntry
	for _, e := range slices.Backward(g.guestBook) {
		if g.ghost || !e.ghost() {
			es = append(es, e)
		}
	}
	return es
}

func (g *Game) updateBook() {
	key := g.in.JustPressed
	switch {
	case key(ebiten.KeySpace):
		g.state = StatePlay
	case key(ebiten.KeyA):
		g.bookTop = max(0, g.bookTop-1)
	case key(ebiten.KeyD):
		g.bookTop = max(0, min(g.bookTop+1, len(g.shown())-bookRows))
	}
}

var outTag = [...]string{"COZY", "DREAD", "FLED", "PASSED ON", "BANISHED"}

// The open book's geometry, px: the cover, the pages inset by bookM, the spine gutter, the ruled lines' pitch.
const (
	bookW, bookH = 880, 440
	bookM        = 10
	spineGap     = 4
	bookPW       = (bookW - 2*bookM - spineGap) / 2
	bookPH       = bookH - 2*bookM
	bookRuled    = 30                                  // the ruled lines' pitch: the hand sits on them (T381 refine 6)
	bookLine0    = 70                                  // the first ruled line, from the cover's top
	bookLines    = (bookPH-40-bookLine0)/bookRuled + 1 // ruled lines a page
	bookBase     = 13                                  // the debug font's baseline row: the hand's y is the line's minus this, scaled
)

var bookBG *ebiten.Image

// bookBackdrop: the two ruled pages on a soft black cover, drawn once column by column. Near the spine each column
// darkens and its top and bottom edges move in, which is what makes the paper read as curving into the gutter; the
// cover is a thin rim that bends with them (T381 refine 3: a soft-cover book, not a board behind it).
func bookBackdrop() *ebiten.Image {
	if bookBG != nil {
		return bookBG
	}
	img := ebiten.NewImage(bookW, bookH)
	cover := color.RGBA{0x16, 0x14, 0x1a, 255}
	rim := func(x, off int) { // the cover's column at x, round a page column bowed in by off: 5 px over, 9 under
		rect(img, x, bookM+off-5, 1, bookPH-2*off+14, cover)
	}
	for x := bookM + bookPW; x < bookM+bookPW+spineGap; x++ { // the spine, where the pages bow in the most
		rim(x, 16)
	}
	shade := func(c color.RGBA, f float64) color.RGBA {
		return color.RGBA{uint8(float64(c.R) * f), uint8(float64(c.G) * f), uint8(float64(c.B) * f), 255}
	}
	paper, ruled := color.RGBA{0xf5, 0xf0, 0xe0, 255}, color.RGBA{0xb4, 0xc2, 0xda, 255}
	for side := range 2 {
		x0 := bookM + side*(bookPW+spineGap)
		for c := range bookPW {
			d := c // distance to the spine
			if side == 0 {
				d = bookPW - 1 - c
			}
			t := max(0, 1-float64(d)/110)
			u := max(0, 1-float64(d)/90)
			f := 1 - 0.34*u*u // the gutter shadow
			if c == 0 && side == 0 || c == bookPW-1 && side == 1 {
				f *= 0.8 // the outer page edge
			}
			off := int(16 * t * t) // the page's top and bottom edges bow in toward the spine
			x, y0, h := x0+c, bookM+off, bookPH-2*off
			rim(x, off)
			if c == 0 && side == 0 || c == bookPW-1 && side == 1 { // the cover's fore edge, 6 px past the page
				for e := 1; e <= 6; e++ {
					rim(x+(2*side-1)*e, off)
				}
			}
			rect(img, x, y0, 1, h, shade(paper, f))
			for k := range bookLines {
				if ly := bookLine0 + k*bookRuled; ly > y0 && ly < y0+h {
					rect(img, x, ly, 1, 1, shade(ruled, f))
				}
			}
		}
	}
	bookBG = img
	return img
}

func (g *Game) drawBookPage(s *ebiten.Image) {
	rect(s, 0, 0, screenW, screenH, color.RGBA{0, 0, 0, 150})
	x, y := (screenW-bookW)/2, (screenH-bookH)/2
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Translate(float64(x), float64(y))
	s.DrawImage(bookBackdrop(), op)
	lx, rx := x+bookM+30, x+bookM+bookPW+spineGap+22
	line := func(k int) int { return y + bookLine0 + k*bookRuled } // ruled line k's y
	g.tintedAt(s, "GUEST BOOK", lx, line(0)-3*bookBase, 3, storeInk, 1)
	es := g.shown()
	if len(es) == 0 {
		scrawl(s, "No entries yet.", lx, line(1), 0, 0, storeDim)
	}
	// The entries flow down the left page under the title, then the right one: a line for the name, the words on
	// the next, a blank line between two.
	side, k, n := 0, 1, 0
	for _, e := range es[min(g.bookTop, len(es)):] {
		words := e.Line
		if words == "" {
			words = scribble
		}
		ls := wrapAt("\""+words+"\"", (bookPW-60)/(2*glyphW))
		if k+1+len(ls) > bookLines {
			if side == 1 {
				break
			}
			side, k = 1, 0
		}
		px := lx
		if side == 1 {
			px = rx
		}
		g.writeEntry(s, e, ls, px, line(k), px+bookPW-52)
		k, n = k+len(ls)+2, n+1
	}
	if len(es) > n {
		pos := strconv.Itoa(g.bookTop+1) + "-" + strconv.Itoa(g.bookTop+n) + " OF " + strconv.Itoa(len(es))
		g.tintedAt(s, pos, rx+bookPW-74-len(pos)*glyphW, y+bookM+22, 1, storeDim, 1)
	}
	g.tintedAt(s, "A/D SCROLL   SPACE OR ESC CLOSE", lx, y+bookM+bookPH-18, 1, storeDim, 1)
}

// bookInks: the pens a human writes with; a ghost writes in the Ectoplasm accent.
var bookInks = [...]color.RGBA{storeInk, {0x23, 0x3a, 0x7a, 255}, {0x5a, 0x34, 0x22, 255}}

// writeEntry writes e by hand: its name on the ruled line at y, its words on the next ones, the day and how it
// went at the right. Each hand is its own, from a hash of the entry (no rng): its pen, a nudge along the line, and
// how it keeps to the line, straight, tilted a little, or bobbing word by word; a guest who fled always bobs.
func (g *Game) writeEntry(s *ebiten.Image, e bookEntry, ls []string, x, y, right int) {
	h := fnv.New32a()
	h.Write([]byte(e.Name + e.Line + strconv.Itoa(e.Day)))
	v := h.Sum32()
	ink := color.Color(bookInks[v%3])
	if e.ghost() {
		ink = storeAccent
	}
	x += int(v>>4%7) - 3
	var tilt float64
	var wob uint32
	switch style := v >> 8 % 4; {
	case style == 3 || e.Out == outFled:
		wob = v>>12 | 1
	case style == 2:
		tilt = float64(int(v>>12%9)-4) * 0.008 // up to about 1.8 degrees either way
	}
	scrawl(s, e.Name, x, y, tilt, wob>>16, ink)
	tag := "DAY " + strconv.Itoa(e.Day) + "  " + outTag[e.Out]
	g.tintedAt(s, tag, right-len(tag)*glyphW, y-bookBase, 1, ink, 1)
	for i, l := range ls {
		scrawl(s, l, x+8, y+(i+1)*bookRuled, tilt, wob>>(i%4*4), ink)
	}
}

// scrawl writes v at twice the font's size by hand on the ruled line at y, from x: tilted by rad around its start,
// or, with wob set, each word bobbing up to 2 px off the line by wob's bits.
func scrawl(s *ebiten.Image, v string, x, y int, rad float64, wob uint32, c color.Color) {
	if wob == 0 {
		var geo ebiten.GeoM
		geo.Scale(2, 2)
		geo.Translate(0, -2*bookBase)
		geo.Rotate(rad)
		geo.Translate(float64(x), float64(y))
		printGeo(s, v, geo, c, 1)
		return
	}
	for i, w := range strings.Split(v, " ") {
		printAt(s, w, x, y-2*bookBase+int(wob>>(i%8*2)%5)-2, 2, c, 1)
		x += (len(w) + 1) * 2 * glyphW
	}
}

// drawBookProp: the book lying on the counter, a small vector one (no cut of the sprite sheet fits): the black cover,
// its white page edge, the spine. In ghost the edge reads in the Ectoplasm ink.
func (g *Game) drawBookProp(s *ebiten.Image, ox, oy int) {
	r := g.bookRect()
	cover, edge := color.RGBA{0x16, 0x14, 0x1a, 255}, color.RGBA{0xf5, 0xf0, 0xe0, 255}
	if g.ghost {
		cover, edge = color.RGBA{0x0c, 0x24, 0x1e, 255}, color.RGBA{0x7a, 0xe8, 0xc0, 255}
	}
	x, y := (r.Min.X+4)*zoom-ox, (r.Min.Y+3)*zoom-oy
	rect(s, x, y, 8*zoom, 5*zoom, cover)
	rect(s, x+2*zoom, y+zoom, 6*zoom, 3*zoom, edge)
	rect(s, x, y, 2*zoom, 5*zoom, cover)
}

// DevBook is main's -book: 1 the counter, you behind it; 2 the page open, filled with a stay of each outcome.
func (g *Game) DevBook(n int) {
	if n < 1 {
		return
	}
	b := g.hotel.Book
	g.player.X, g.player.Y = float64(b.Min.X+world.TileSize/2), float64(b.Min.Y+world.TileSize+world.TileSize/2)
	w, h := g.mapSize()
	g.cam.Snap(g.player.X, g.player.Y, w, h)
	if n < 2 {
		return
	}
	g.guestBook = append(g.guestBook,
		bookEntry{"Magda Feltz", namedHumans[0].Visit[0], 1, outCozy}, bookEntry{"Teo Barros", namedHumans[1].Dread, 1, outDread},
		bookEntry{"Noor Haddad", namedHumans[3].Fled, 2, outFled}, bookEntry{"Ana Costa", notebookCozy[0], 2, outCozy},
		bookEntry{"Agatha Fogg", notebookPassed[0], 2, outPassed}, bookEntry{"Silas Wick", notebookBanished[1], 3, outBanished},
		bookEntry{"Osvaldo Feltz", namedGhosts[0].Visit[0], 3, outPassed}, bookEntry{"Hugo Pike", "", 3, outFled})
	g.openBook()
}
