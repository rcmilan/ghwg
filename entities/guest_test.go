package entities

import (
	"image"
	"testing"
)

// D89: a guest pushed past a waypoint along its next leg keeps going forward instead of turning back;
// one pushed off to the side still walks to it, so it never cuts a corner.
func TestFollowPathPastWaypoint(t *testing.T) {
	path := []image.Point{{8, 8}, {8, 24}}
	g := NewGuest(image.Pt(8, 14), "Adam") // 6 px past (8,8), toward (8,24)
	g.SetPath(path)
	g.FollowPath(0.5)
	if len(g.Path) != 1 || g.Y < 14 {
		t.Errorf("pushed past the waypoint: path %v, y %.1f; want it dropped and the guest moving on", g.Path, g.Y)
	}
	g = NewGuest(image.Pt(14, 10), "Adam") // 6 px to the side of the leg: not on it
	g.SetPath(path)
	g.FollowPath(0.5)
	if len(g.Path) != 2 {
		t.Errorf("off to the side: path %v; want the waypoint kept", g.Path)
	}
}

// A pair's mate and a Room mirror's ghost both trail the pair's leader (08 B1): they never bump either.
func TestMateSiblings(t *testing.T) {
	lead := &Guest{}
	mate, through, stranger := &Guest{Leader: lead}, &Guest{Leader: lead}, &Guest{}
	if !mate.Mate(through) || !through.Mate(mate) || !mate.Mate(lead) || stranger.Mate(mate) || stranger.Mate(&Guest{}) {
		t.Fatal("siblings of one leader must be mates, strangers not")
	}
}
