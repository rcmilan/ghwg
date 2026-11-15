package game

import "ghosthotel/entities"

// Room Mood (07 "Room axis"): one number per Room, -100 (dread) .. 100 (cozy). Dirt, pay, the floor's tint
// and the Priests' work all read it. It moves only on events (a serve, a clean, mess, a haunt), never drifts.
// Dirt is its face: spots from moodSpots down, in the scale the spot tuning (spotUnit, mess, the calm lines)
// is written in, converted here. All values are placeholders.
const (
	moodMin     = -100 // the Room is Closed
	moodMax     = 100
	moodSpots   = 50        // spots show below this; a clean raises the Mood to it
	moodStart   = moodSpots // a new Run's Rooms: spotless, not yet cozy (Z1)
	moodClean   = moodSpots
	moodServe   = 25 // a serve answered at once; falls to 0 as the ask nears expiry
	moodExpire  = 10 // an expired ask
	intrudeMood = 1  // Mood the Concierge costs each intrudeEvery game minutes he spends in a Room with its Guests
	// 10, not every minute (2026-11-14, 4c): at a 4 minute day a game minute is a sixth of a second, and a four
	// second walk in to clean cost the Room 25 Mood, more than the clean gave back.
	intrudeEvery = 10
	intrudeGrace = 30                              // game minutes he may spend in there first (5 s): a clean is not lingering, and no longer leaves a stain of its own
	rotMood      = -33                             // from here a Room rots
	dirtMax      = (3*(moodSpots-moodMin) + 2) / 4 // the dirt level of moodMin

	hauntBase        = 5 // Mood a haunt costs before Phantasm
	hauntPerPhantasm = 1 // and per point of it
)

func newMoods(n int) []int {
	m := make([]int, n)
	for i := range m {
		m[i] = moodStart
	}
	return m
}

// roomMood: the Room's Mood; a Room that doesn't exist is cozy.
func (g *Game) roomMood(room int) int {
	if room < 0 || room >= len(g.mood) {
		return moodMax
	}
	return g.mood[room]
}

// addMood moves the Room's Mood by d, clamped to -100..100.
func (g *Game) addMood(room, d int) {
	if room >= 0 && room < len(g.mood) {
		was := g.mood[room]
		g.mood[room] = max(moodMin, min(moodMax, was+d))
		if was > moodMin && g.mood[room] == moodMin { // once, as it reaches -100 (08 B1-N)
			g.notice(g.hotel.Rooms[room].Number + " CLOSED")
		}
	}
}

// moodOf and dirtOf convert between Mood and dirt, the spots' scale: none from moodSpots up, then one spot
// unit per ~3.75 Mood points (dirt = 3/4 of the points below moodSpots). An exact round trip for 0..dirtMax.
func moodOf(dirt int) int { return max(moodMin, min(moodSpots, moodSpots-dirtMood(dirt))) }
func dirtOf(mood int) int { return max(0, (3*(moodSpots-mood)+2)/4) }

// dirtMood: the Mood points d dirt weighs.
func dirtMood(d int) int { return (4*d + 1) / 3 }

func (g *Game) dirtLevel(room int) int { return dirtOf(g.roomMood(room)) }

// setDirtLevel sets the Room's Mood to the one that shows dirt (dev flags and tests).
func (g *Game) setDirtLevel(room, dirt int) {
	if room >= 0 && room < len(g.mood) {
		g.mood[room] = moodOf(max(0, dirt))
	}
}

// addDirtLevel: guest mess and frights, d dirt more, lower the Mood by its weight.
func (g *Game) addDirtLevel(room, d int) { g.addMood(room, -dirtMood(d)) }

// raiseMood lifts the Room's Mood to at least to; it never lowers it.
func (g *Game) raiseMood(room, to int) { g.addMood(room, max(0, to-g.roomMood(room))) }

// moodPay: what the Room's Mood does to what its Guests pay: 0 at -100, 1 at 0, 2 at +100, linear.
func (g *Game) moodPay(room int) Num { return Num(100+g.roomMood(room)) / 100 }

// dread: how far the Room's Mood is from cozy, 0 at +100 to 1 at -100 (the floor's shift, the Shadow's pace,
// the calm lines, the supply's pay).
func (g *Game) dread(room int) float64 {
	return float64(moodMax-g.roomMood(room)) / (moodMax - moodMin)
}

// haunts: a haunt in the Room lowers its Mood by damage.
func (g *Game) haunts(room, damage int) { g.ghostDirt(room, damage) }

// ghostDirt lowers the Room's Mood by d as a ghost's doing (a haunt, a fright): the dirt that shows for it is
// spiritual (dirt.go).
func (g *Game) ghostDirt(room, d int) {
	if room < 0 || room >= len(g.mood) {
		return
	}
	g.spots(room) // what was owed before is not the ghost's
	g.addMood(room, -d)
	g.byGhost = true
	g.spots(room)
	g.byGhost = false
}

// ghostHauntDamage: the Mood a ghost's haunt costs its Room, from its Phantasm.
func ghostHauntDamage(ghost *entities.Guest) int { return hauntBase + hauntPerPhantasm*ghost.Phantasm }

// spent: the ghost has done all the haunts it had in it, one per point of Phantasm: the weakest haunts once.
// A spent ghost passes on (updateGhost), and its Room books the next (2026-11-14).
func spent(ghost *entities.Guest) bool { return ghost.Haunted >= max(1, ghost.Phantasm) }

// haunt: the ghost haunts its Room; with a human in it, one haunt nearer to spent.
func (g *Game) haunt(room int, ghost *entities.Guest) {
	g.haunts(room, ghostHauntDamage(ghost))
	g.terrify(room, ghost)
	if _, in := g.inside(room); len(in) > 0 { // 08 F0: only a haunt with a human in counts toward passing on
		ghost.Haunted++
	}
}

// hauntTick runs each frame for a ghost in its Room: every ghost.Haunt game minutes it haunts, until it is
// spent. G1 calls it.
func (g *Game) hauntTick(room int, ghost *entities.Guest) {
	if ghost.Haunt > 0 && !spent(ghost) && g.clock%minuteFrames == 0 && (g.clock/minuteFrames-ghost.HauntAt)%ghost.Haunt == 0 {
		g.haunt(room, ghost)
	}
}
