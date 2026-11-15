package game

import (
	"image"
	"image/color"

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

// earn adds n of r and floats "+N" from at (world px).
func (g *Game) earn(r Res, n Num, at image.Point) {
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
