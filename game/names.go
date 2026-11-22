package game

import "strings"

// Every named character's display name lives here: a rename touches one line. Keys ("magenta", "tim"...) are the
// cast's identifiers, not names; they stay as they are.
const (
	// The questline cast (circle.go, cast.go).
	nameMagenta        = "Magenta" // the witch; also her sprite sheet's base name (Magenta_*.png)
	nameMagentaFull    = "Magenta Moss"
	nameCelestine      = "Celestine"
	nameMadame         = "Madame" // what the cast calls Celestine
	nameCelestineFull  = "Madame Celestine Arbogast"
	nameCelestineShort = "Mme Arbogast" // short enough to leave the book entry's tag room
	nameArbogast       = "Madame Arbogast"
	nameMagda          = "Magda"
	nameMagdaFull      = "Magda Feltz"
	nameOsvaldo        = "Osvaldo"
	nameOsvaldoFull    = "Osvaldo Feltz"
	nameTim            = "Tim"
	nameTimFull        = "Tim Okafor"
	nameTeo            = "Teo"
	nameTeoFull        = "Teo Barros"

	// The other named humans (cast.go).
	nameGusFull     = "Gus Pimentel"
	nameNoorFull    = "Noor Haddad"
	nameBeaFull     = "Bea Marchetti"
	nameLouFull     = "Lou Marchetti"
	nameWendellFull = "Wendell Crane"

	// The Fathers.
	nameFatherK = "Father K"
	nameFatherM = "Father M"
	nameFatherP = "Father P"

	// The MacGuffin, and Magenta's item the Terminal calls by name.
	nameMacguffin    = "The Eternal Eldritch MacGuffin"
	nameGerald       = "Gerald"
	nameShowDontTell = "Show, don't tell" // the crystal ball (itemBall)
)

// upper: a name in a row's capitals.
func upper(s string) string { return strings.ToUpper(s) }
