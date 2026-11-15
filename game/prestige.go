package game

// 08 W3 (P1) seam, owned by P1-C: the prestige itself. Bodies are stubs until P1-C fills them; signatures stay.

// resetRun clears everything a prestige takes away (everything but level and bonus) and starts the next Run:
// Day 1 08:00, the Concierge in the Entrance hall, the start grid, a new seed, no title or intro.
func (g *Game) resetRun() {}

// startPrestige banks the Run into the bonus, raises the level, saves (saveGame) and calls resetRun.
// The cutscene (playPrestige, prestige_reel.go) calls it when it ends.
func (g *Game) startPrestige() { g.resetRun() }
