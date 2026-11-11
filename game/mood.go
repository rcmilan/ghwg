package game

// Room Mood (07 "Room axis"): one number per Room, -100 (dread) .. 100 (cozy). Dirt, tips, the floor's tint
// and the Priests' work all read it. 06 had it as 0 (calm) .. 150 (Closed), now called dirt: Mood = 100 - dirt × 4/3.
// The dirt tuning (spots, stains, mess, the calm lines) stays in that scale, converted here once.
const (
	moodMin = -100 // the Room is Closed
	moodMax = 100
	rotMood = -33 // from here a Room rots (dirt 100)
	dirtMax = 150 // the dirt level of moodMin
)

func newMoods(n int) []int {
	m := make([]int, n)
	for i := range m {
		m[i] = moodMax
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
		g.mood[room] = max(moodMin, min(moodMax, g.mood[room]+d))
	}
}

// moodOf and dirtOf convert between Mood and the dirt scale (an exact round trip for every dirt 0..150).
func moodOf(dirt int) int { return max(moodMin, min(moodMax, 100-(4*dirt+1)/3)) }
func dirtOf(mood int) int { return (3*(100-mood) + 2) / 4 }

func (g *Game) dirtLevel(room int) int { return dirtOf(g.roomMood(room)) }

func (g *Game) setDirtLevel(room, dirt int) {
	if room >= 0 && room < len(g.mood) {
		g.mood[room] = moodOf(max(0, dirt))
	}
}

func (g *Game) addDirtLevel(room, d int) { g.setDirtLevel(room, g.dirtLevel(room)+d) }
