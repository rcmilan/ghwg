package game

import (
	"image"
	"image/color"
	"slices"

	"github.com/hajimehoshi/ebiten/v2"
)

// Res is one of the two currencies (D178). Fear and Room Mood are stats, not currencies.
type Res int

// String is the currency's display name.
func (r Res) String() string { return [...]string{"Money", "Ectoplasm"}[r] }

const (
	Money Res = iota
	Ecto
)

// Cost is a price in both currencies: Cost{Money: 10} or Cost{Money: 5, Ecto: 3}.
type Cost [2]Num

// noFloat: earn without a floater (the haunt batches its own "+N", haunt.go).
var noFloat = image.Pt(-1, -1)

// earn adds n of r (times the bonus) and floats "+N" from at (world px).
func (g *Game) earn(r Res, n Num, at image.Point) {
	if r == Ecto {
		g.runEcto += n // before the bonus: the prestige grows the bonus from it (prestige.go)
	}
	n *= Num(g.bonus)           // the prestige bonus, on every income: the one place
	if g.earned == ([2]Num{}) { // a Run's first earning: the last Run's are no rate of this one
		g.recent = nil
	}
	g.recent = append(slices.DeleteFunc(g.recent, g.stale), earning{g.clock, r, n})
	g.wallet[r] += n
	g.earned[r] += n // lifetime: opens store items (shop.go)
	if at == noFloat {
		return
	}
	txt := "+" + fmtNum(n)
	if r == Money {
		txt = "+$" + fmtNum(n)
	}
	g.floaters = append(g.floaters, floater{float64(at.X), float64(at.Y), txt, g.tick, floaterKind(r)}) // floatMoney, floatEcto
}

// earning: one earn, at its clock: the Terminal's rates (08 F0-T).
type earning struct {
	at int
	r  Res
	n  Num
}

// minuteRate: a real minute's frames (60 TPS) over a game hour's: an hour's earnings times this is a minute's.
const minuteRate = 60 * 60 / hourFrames

// stale: an earning outside the last game hour.
func (g *Game) stale(e earning) bool { return e.at <= g.clock-hourFrames || e.at > g.clock }

// perMinute: Money and Ecto earned a real minute, averaged over the last game hour (the Terminal's header).
func (g *Game) perMinute() Cost {
	var c Cost
	if g.earned == ([2]Num{}) { // nothing yet this Run
		return c
	}
	for _, e := range g.recent {
		if !g.stale(e) {
			c[e.r] += e.n * minuteRate
		}
	}
	return c
}

func (g *Game) afford(c Cost) bool { return g.wallet[Money] >= c[Money] && g.wallet[Ecto] >= c[Ecto] }

// pay takes the whole Cost, or nothing if the wallet can't cover it.
func (g *Game) pay(c Cost) bool {
	if !g.afford(c) {
		return false
	}
	g.wallet[Money], g.wallet[Ecto] = g.wallet[Money]-c[Money], g.wallet[Ecto]-c[Ecto]
	return true
}

// resInk: a currency's color, cycling slowly (hud.go): gold↔silver for Money, green↔purple for Ecto.
func resInk(r Res) color.Color { return inkAt(r, ebiten.Tick()) }
