package game

// prefersStrongGhost: a Strategy's read of a revealed Party (cast.md): a Skeptic never flees and a Medium farms
// Ectoplasm, so both go to the strongest ghost's Room. An unrevealed Party tells nothing.
// ponytail: not wired yet; CARD's REVEAL verb decides when a Strategy may call it (flagged for the merge).
func prefersStrongGhost(p *Party) bool {
	return p.revealed && (hasQuirk(p, Skeptic) || hasQuirk(p, Medium))
}
