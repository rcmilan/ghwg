package entities

// Quirk is a guest's one rule (context/08-quests/cast.md). The rules live in game/quirk.go; the names are
// game/cast.go's. The type is here so Identity can sit on the Guest.
type Quirk uint8

const (
	NoQuirk Quirk = iota
	// Human quirks.
	Medium
	Skeptic
	BigTipper
	NightOwl
	NeatFreak
	// Ghost quirks.
	ShowOff
	Shy
	Jealous
	Gentle
	Homebody
)

// Identity is who a guest is, rolled when its Party books (game/cast.go). A human has Age and Job; a ghost has
// Died (a year) and Cause. The zero Identity is an unrolled guest (tests, staff).
type Identity struct {
	Name    string
	Age     int      // a human's; 0 when AgeNote says it
	AgeNote string   // a human's age said in words ("ask the moon"), else empty
	Job     string   // a human's
	Died    int      // a ghost's death year, 0 for a human
	Cause   string   // a ghost's cause of death
	Lines   []string // what it says on its card: one line an opening, in turn (game/card.go)
	Quirk   Quirk
	Key     string // the named cast's key (cast.go), empty for a generated guest
}
