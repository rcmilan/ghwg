package game

// Task is a kind of tap (D177); a (Room, Task) pair is a job J1 ranks. Later slices append before taskCount.
type Task int

const (
	TaskServe Task = iota
	TaskHousekeep
	taskCount
)

// tapped counts one tap of t in the Room, by the Concierge or a Shadow alike. Every verb calls it once per tap.
func (g *Game) tapped(room int, t Task) { g.taps[room][t]++ }
