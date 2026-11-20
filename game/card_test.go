package game

import (
	"image"
	"image/color"
	"strings"
	"testing"

	"ghosthotel/entities"

	"github.com/hajimehoshi/ebiten/v2"
)

// 08 T375: the guest card. cardRun: two Rooms, a human of Nerve 9 settled in Room 0, the Concierge on its plate, and the card open.
func cardRun(t *testing.T) (*Game, *script, *Party) {
	t.Helper()
	g, s := matchRun(t, 2)
	p := settledHuman(t, g, 0, 9)
	ghostOf(g, 1, 12) // beyond its Nerve: no Room to move to
	g.wallet[Ecto] = 25
	press(g, s, center(g.hotel.Rooms[0].Plate()))
	if g.state != StateTalk || g.parley == nil || g.parley.card == nil {
		t.Fatalf("SPACE on the plate did not open the card (state %d)", g.state)
	}
	return g, s, p
}

func tap(g *Game, s *script, k ebiten.Key) {
	s.hold()
	g.Update()
	s.hold(k)
	g.Update()
	s.hold()
}

func cardText(g *Game) string {
	var ls []string
	for _, r := range g.cardRows() {
		ls = append(ls, r.text)
	}
	return strings.Join(ls, "\n")
}

func TestCardHumanRows(t *testing.T) {
	g, _, p := cardRun(t)
	who := p.Guests[0].Who
	txt := cardText(g)
	for _, want := range []string{who.Name, who.Job, who.Card, "REVEAL  10 ECTO"} {
		if !strings.Contains(txt, want) {
			t.Errorf("unrevealed card lacks %q:\n%s", want, txt)
		}
	}
	if strings.Contains(txt, "NERVE") || strings.Contains(txt, "QUIRK") {
		t.Errorf("unrevealed card shows Nerve or quirk:\n%s", txt)
	}
	if g.scale >= 1 {
		t.Errorf("world not slowed: scale %v", g.scale)
	}
}

func TestCardRevealSpendsOnce(t *testing.T) {
	g, s, p := cardRun(t)
	tap(g, s, ebiten.KeySpace)
	if g.wallet[Ecto] != 15 || !p.revealed {
		t.Fatalf("REVEAL: Ecto %v, revealed %v; want 15, true", g.wallet[Ecto], p.revealed)
	}
	txt := cardText(g)
	if !strings.Contains(txt, "NERVE 9") || !strings.Contains(txt, "QUIRK") || strings.Contains(txt, "REVEAL") {
		t.Errorf("revealed card:\n%s", txt)
	}
	if g.reveal(p) || g.wallet[Ecto] != 15 {
		t.Error("a second REVEAL charged")
	}
}

func TestCardRevealUnaffordable(t *testing.T) {
	g, s, p := cardRun(t)
	g.wallet[Ecto] = revealEcto - 1
	if r := g.cardRows(); r[len(r)-1].text[:6] != "REVEAL" || r[len(r)-1].ink != (color.Gray{120}) {
		t.Errorf("unaffordable REVEAL is not grey: %v", r[len(r)-1])
	}
	tap(g, s, ebiten.KeySpace)
	if p.revealed || g.wallet[Ecto] != revealEcto-1 {
		t.Error("REVEAL went through without the Ecto")
	}
}

func TestCardQuirkRow(t *testing.T) {
	g, _, p := cardRun(t)
	g.reveal(p)
	p.Guests[0].Who.Quirk = NoQuirk
	if !strings.Contains(cardText(g), "NO QUIRK") {
		t.Errorf("no quirk row:\n%s", cardText(g))
	}
	p.Guests[0].Who.Quirk = Skeptic
	if !strings.Contains(cardText(g), "SKEPTIC") {
		t.Errorf("quirk row:\n%s", cardText(g))
	}
}

func TestCardMoveOnlyWhereItApplies(t *testing.T) {
	g, _, p := cardRun(t)
	if g.canMove(p) {
		t.Error("MOVE offered with no Room to move to")
	}
	g.parties[len(g.parties)-1].Guests[0].Phantasm = 1 // Room 1's ghost now fits: a Room to move to
	if !g.canMove(p) {
		t.Error("MOVE not offered with a better Room")
	}
	if os := g.cardOpts(p, p.Guests[0]); len(os) != 2 || os[1] != optMove {
		t.Errorf("options %v; want REVEAL, MOVE", os)
	}
}

func TestCardPairFlips(t *testing.T) {
	g, s, p := cardRun(t)
	mate := entities.NewGuest(image.Pt(int(p.Guests[0].X), int(p.Guests[0].Y)), p.Guests[0].Character)
	mate.Leader, mate.FleeAt = p.Guests[0], p.Guests[0].FleeAt
	g.rollHuman(&mate)
	p.Guests, p.due, p.cents = append(p.Guests, &mate), nil, nil
	first := cardText(g)
	tap(g, s, ebiten.KeyD)
	if g.parley.card.who != 1 || !strings.Contains(cardText(g), mate.Who.Name) || cardText(g) == first {
		t.Errorf("D did not flip to the mate:\n%s", cardText(g))
	}
	tap(g, s, ebiten.KeyA)
	if g.parley.card.who != 0 {
		t.Error("A did not flip back")
	}
}

func TestGhostCard(t *testing.T) {
	g, s := matchRun(t, 2)
	gh := ghostOf(g, 0, 7)
	gh.Guests[0].Who = entities.Identity{Name: "Osvaldo Feltz", Died: 1987, Cause: "the sea", Card: "Seen Magda?", Quirk: Gentle, Key: "osvaldo"}
	g.wallet[Ecto] = 25
	g.ghost = true
	press(g, s, center(g.hotel.Rooms[0].Plate()))
	if g.parley == nil || g.parley.card == nil {
		t.Fatal("the plate did not open the ghost's card in ghost")
	}
	txt := cardText(g)
	if !strings.Contains(txt, "DIED 1987") || !strings.Contains(txt, "the sea") || strings.Contains(txt, "MOVE") || strings.Contains(txt, "PHANTASM") {
		t.Errorf("ghost card:\n%s", txt)
	}
	tap(g, s, ebiten.KeySpace)
	txt = cardText(g)
	if g.wallet[Ecto] != 15 || !strings.Contains(txt, "PHANTASM 7") || !strings.Contains(txt, "GENTLE") || !strings.Contains(strings.ReplaceAll(txt, "\n", " "), ghostFact("osvaldo")[:20]) {
		t.Errorf("revealed ghost card (Ecto %v):\n%s", g.wallet[Ecto], txt)
	}
}

func TestNamedRevealPersists(t *testing.T) {
	g, s, p := cardRun(t)
	p.Guests[0].Who.Key = "magda"
	tap(g, s, ebiten.KeySpace)
	g.endTalk()
	q := settledHuman(t, g, 1, 9) // a later visit: a new Party, the same name
	q.Guests[0].Who.Key = "magda"
	if !g.revealed(q, q.Guests[0]) || q.revealed {
		t.Error("a named guest's reveal was not kept for its next Party")
	}
	if ks := g.revealedList(); len(ks) != 1 || ks[0] != "magda" {
		t.Errorf("revealedList %v", ks)
	}
	h, _ := headless(t, 2, 1, 1)
	h.resume(&saveData{Level: 0, Bonus: 1, Revealed: g.revealedList()})
	if !h.revealedKeys["magda"] {
		t.Error("the save did not bring the reveal back")
	}
}

// T381 refine 2: TALK shows on the plate beside a Room's door, in either Dimension, and opens its Party's card.
func TestTalkOpensCardAtPlate(t *testing.T) {
	for _, ghost := range []bool{false, true} {
		g, s := matchRun(t, 2)
		settledHuman(t, g, 0, 9)
		ghostOf(g, 0, 3)
		ghostOf(g, 1, 3) // no Room a ghost-free Party would rather have: no MOVE either way
		standIn(g, 0)
		g.ghost = ghost
		want := g.cardParty(0)
		if g.talker() != want || want == nil {
			t.Fatalf("ghost %v: no TALK at the plate", ghost)
		}
		keys(g, s, ebiten.KeySpace)
		if g.state != StateTalk || g.parley.card == nil || g.parley.p != want {
			t.Fatalf("ghost %v: SPACE did not open the Party's card (state %d)", ghost, g.state)
		}
	}
}
