package game

import "math"

// 08 W4 (T373 seam, T374 UP): the 8 leveled upgrades of context/08-quests/upgrades.md. g.up[u] is the level
// bought (Money only, reset at the prestige, saved in the Run). No caps, each level 10% weaker than the last;
// Bigger shelves is capped at +3. The hooks read them through upMul, upStock and upPrice, which with every level
// at 0 are neutral (x1, +0). UP adds the Terminal rows and the buying.

type Upgrade int

const (
	LobbyDecor    Upgrade = iota // +% on every Room's Rate
	Brochures                    // -% on bookGap
	Soundproofing                // -% Mood lost per haunt
	SeanceTable                  // +% Ectoplasm per sighting
	TipJar                       // +% on checkout tips
	CleaningKit                  // -% on the time a clean takes
	BiggerShelves                // +1 on each Pantry's stock per Supply, capped
	OfferingBox                  // +% Ectoplasm per Priest ritual
	upgradeCount
)

type upKind int

const (
	upBoost   upKind = iota // 1 + e(1 - 0.9^N)/0.1
	upReducer               // (1 - e)^N
	upStockK                // +min(N, Cap)
)

// upDecay: each level is this much of the one before it.
const upDecay = 0.9

type upgradeDef struct {
	Name   string
	Base   Num     // price of level 1, before the Rooms-owned factor
	Effect float64 // level 1's effect: the boost or the cut, a share; Bigger shelves: stock
	Kind   upKind
	Cap    int // highest level, 0 = none
}

var upgrades = [upgradeCount]upgradeDef{
	LobbyDecor:    {"Lobby decor", 150, 0.08, upBoost, 0},
	Brochures:     {"Brochures", 120, 0.20, upReducer, 0},
	Soundproofing: {"Soundproofing", 200, 0.20, upReducer, 0},
	SeanceTable:   {"Seance table", 250, 0.20, upBoost, 0},
	TipJar:        {"Tip jar", 180, 0.20, upBoost, 0},
	CleaningKit:   {"Cleaning kit", 160, 0.30, upReducer, 0},
	BiggerShelves: {"Bigger shelves", 200, 1, upStockK, 3},
	OfferingBox:   {"Offering box", 220, 0.80, upBoost, 0},
}

// upMulAt: the multiplier of upgrade u at level n. A boost is 1 + e(1-0.9^n)/0.1 (toward 1 + 10e), a reducer
// (1-e)^n. 1 at level 0, and for a stock upgrade (use upStockAt).
func upMulAt(u Upgrade, n int) float64 {
	d := upgrades[u]
	n = max(0, n)
	if d.Cap > 0 {
		n = min(n, d.Cap)
	}
	switch d.Kind {
	case upBoost:
		return 1 + d.Effect*(1-math.Pow(upDecay, float64(n)))/(1-upDecay)
	case upReducer:
		return math.Pow(1-d.Effect, float64(n))
	}
	return 1
}

// upStockAt: the extra Pantry stock at Bigger shelves level n, capped.
func upStockAt(n int) int { return max(0, min(n, upgrades[BiggerShelves].Cap)) }

// upPriceAt: the price of level n+1 of u with `rooms` Rooms owned: base x (n+1) x the per-Room factor of
// keys and growth (roomPrice).
func upPriceAt(u Upgrade, n, rooms int) Num { return roomPrice(upgrades[u].Base*Num(n+1), rooms) }

// upMul: the Run's multiplier of upgrade u (boost: above 1; reducer: below 1).
func (g *Game) upMul(u Upgrade) float64 { return upMulAt(u, g.up[u]) }

// upStock: the extra stock each Pantry holds per Supply (Bigger shelves).
func (g *Game) upStock() int { return upStockAt(g.up[BiggerShelves]) }

// upPrice: the price of u's next level.
func (g *Game) upPrice(u Upgrade) Num { return upPriceAt(u, g.up[u], g.ownedCount()) }

// pantryMax: how many of each Supply a Pantry holds.
func (g *Game) pantryMax() int { return pantryStockMax + g.upStock() }

// ritualEctoBase: the Ectoplasm every Priest ritual pays, before the Offering box and the prestige bonus.
const ritualEctoBase = 10

// ritualEcto: the Ectoplasm a Priest's finished ritual pays, before the prestige bonus (earn adds it).
func (g *Game) ritualEcto(pr *priest) Num { return ritualEctoBase * Num(g.upMul(OfferingBox)) }

// scaleInt: n times m, rounded: a hook's int values (m of 1 gives n).
func scaleInt(n int, m float64) int { return int(math.Round(float64(n) * m)) }

// bookGapNow: game minutes from a Party leaving to the next one's arrival (bookGap, Brochures).
func (g *Game) bookGapNow() int { return scaleInt(bookGap, g.upMul(Brochures)) }

// cleanFrames: frames a clean of base frames takes, the Concierge's and the housekeepers' (Cleaning kit). A clean
// has no duration but its fade (dirt.go), so that is all the kit shortens (T374: the housekeepers' work is instant too).
func (g *Game) cleanFrames(base int) int { return max(1, scaleInt(base, g.upMul(CleaningKit))) }
