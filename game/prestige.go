package game

import (
	"fmt"
	"image/color"
	"math"
	"math/rand/v2"
	"slices"

	"ghosthotel/assets"

	"github.com/hajimehoshi/ebiten/v2"
)

// 08 W3 (P1-C): the prestige. Father P, a STAFF row of the Terminal, sells it once the hotel is at its largest
// and every Room is open. The Run's Ectoplasm and the Money left over become a permanent income bonus, the
// grid's limit rises, and the next Run starts from nothing. Placeholders, the sim tunes them.
const (
	rowFatherP     Unlock = "father-p" // the Service's ID only names it
	prestigeEcto          = 300        // Ectoplasm: Father P's fee
	prestigeDonate        = 10         // Money that counts as 1 Ectoplasm in the bonus: $1000 is 100 Ecto
	prestigeUnit          = 6000       // Ectoplasm (donated Money included) that gives +1 of bonus (08 W3 sim: keeps a Run near 30 min)
	macguffinBonus        = 0.25       // 08 W5: the one Bonus step the MacGuffin's summoning adds, once ever (placeholder)
)

// fatherPLine: what Father P adds on his confirm card about the questline (08 W5): once beat 3 has ever been done, and
// after the finale, "" before.
func (g *Game) fatherPLine() string {
	switch {
	case g.questRec.MacGuffin:
		return fatherPQuest[1]
	case g.timesDone(3) > 0:
		return fatherPQuest[0]
	}
	return ""
}

// prestigeReady: the hotel is at its largest and every Room is owned.
func (g *Game) prestigeReady() bool {
	c, r := g.hotel.Grid()
	return c == maxGrid(g.level) && r == maxGrid(g.level) && g.ownedCount() == len(g.hotel.Rooms)
}

// prestigeGain: what this Run adds to the bonus: the square root of its Ectoplasm (before the bonus) and
// donated Money over prestigeUnit, so each prestige pays less per Ectoplasm than the one before.
func (g *Game) prestigeGain() float64 {
	return math.Sqrt((float64(g.runEcto) + float64(g.wallet[Money])/prestigeDonate) / prestigeUnit)
}

// askPrestige is SPACE on Father P's row (shop row i): the confirm card opens, or why not.
func (g *Game) askPrestige(i int) string {
	switch {
	case !g.onSale(i):
		return "COMING SOON"
	case !g.afford(Cost{Ecto: prestigeEcto}):
		return "CAN'T AFFORD"
	}
	g.prestigeAsk = true
	return ""
}

// confirmPrestige is SPACE on the open card: pay Father P and play the cutscene, which ends in startPrestige.
func (g *Game) confirmPrestige() {
	g.prestigeAsk = false
	if g.pay(Cost{Ecto: prestigeEcto}) {
		g.playPrestige()
	}
}

// startPrestige banks the Run into the bonus, raises the level, saves (saveGame) and calls resetRun.
// The cutscene (playPrestige, prestige_reel.go) calls it when it ends.
func (g *Game) startPrestige() {
	g.bonus += g.prestigeGain()
	g.level++
	g.saveGame()
	g.resetRun()
}

// resetRun clears everything a prestige takes away (everything but level and bonus) and starts the next Run:
// Day 1 08:00, the Concierge in the Entrance hall, the start grid, a new seed, no title or intro.
func (g *Game) resetRun() {
	seed := rand.Uint64()
	g.rng, g.chat, g.words = rand.New(rand.NewPCG(seed, seed)), rand.New(rand.NewPCG(seed, seed+1)), rand.New(rand.NewPCG(seed, seed+2))
	g.seedIdent(seed)
	g.up = [upgradeCount]int{}
	stayed := g.macguffinParty() != nil          // 08 W5: its stay is signed at the Run's end, the next notebook's first entry
	g.guestBook, g.met, g.leftAt = nil, nil, nil // the notebook is the Run's; the visits stay
	g.runEcto, g.prestigeAsk, g.termTab, g.termPick = 0, false, 0, 0
	intro := g.introOn
	g.introOn = false
	g.startRun() // rebuilds the start grid (ungrow) and everything a Run owns
	g.introOn = intro
	if stayed {
		g.guestBook = []bookEntry{{macguffinCard.Name, macguffinCard.Book, day(g.clock), outCozy}}
	}
	g.notice(fmt.Sprintf("LEVEL %d   BONUS X%.2f", g.level, g.bonus))
}

// drawPrestigeCard: the confirm card over the Terminal: what Father P gives and what he takes.
func (g *Game) drawPrestigeCard(s *ebiten.Image) {
	rect(s, 0, 0, screenW, screenH, color.RGBA{0, 0, 0, 150})
	const w, h = 520, 250
	x, y := (screenW-w)/2, (screenH-h)/2
	rect(s, x, y, w, h, storeCard)
	rect(s, x, y, w, 6, storeAccent)
	x += 20
	g.tintedAt(s, "FATHER P", x, y+20, 3, storeInk, 1)
	g.tintedAt(s, fmt.Sprintf("YOU GAIN   LEVEL %d: A %dx%d HOTEL", g.level+1, maxGrid(g.level+1), maxGrid(g.level+1)), x, y+70, 1, storeEcto, 1)
	g.tintedAt(s, fmt.Sprintf("            BONUS X%.2f -> X%.2f ON ALL INCOME", g.bonus, g.bonus+g.prestigeGain()), x, y+90, 1, storeEcto, 1)
	g.tintedAt(s, "YOU LOSE   THE HOTEL, MONEY, ECTOPLASM,", x, y+130, 1, assets.Locked, 1)
	g.tintedAt(s, "            UNLOCKS AND STAFF. LEVEL AND BONUS STAY.", x, y+150, 1, assets.Locked, 1)
	for i, l := range wrapAt(g.fatherPLine(), 78) {
		g.tintedAt(s, l, x, y+172+i*14, 1, storeDim, 1)
	}
	g.tintedAt(s, "SPACE PAY", x, y+h-44, 2, storeInk, 1)
	g.drawPrice(s, Cost{Ecto: prestigeEcto}, x+9*2*glyphW+12, y+h-44, 2, false)
	g.tintedAt(s, "ESC BACK", x+w-40-8*glyphW, y+h-40, 1, storeDim, 1)
}

// DevPrestigeReady is main's -prestige-ready: a full level-1 hotel, every key bought, the Terminal open on Father P's
// row with the means to pay and (card) his confirm card open.
func (g *Game) DevPrestigeReady(card bool) {
	g.level, g.bonus = 1, 1.8
	g.DevBusy()
	g.busy, g.runEcto, g.wallet = false, 3000, [2]Num{2000, 400}
	g.openTerminal()
	g.termTab = tabStaff
	g.termPick = slices.IndexFunc(tabItems(tabStaff), func(i int) bool { return shop[i].ID == rowFatherP })
	g.prestigeAsk = card
}
