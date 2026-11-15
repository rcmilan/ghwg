package game

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// U2: the game has no message line. All it has to say to the player goes through notify and shows on the HUD
// (hud.go); nothing else may hold or print a notification. This reads the package's sources, so a change that
// brings one back fails here, whoever writes it:
//   - Game has no message field, and nothing reads or writes one;
//   - the notice is touched only in hud.go, through notify and said everywhere else;
//   - drawFrame prints no text of its own: what it draws last is the HUD;
//   - the debug font is printed by printAt alone, and the white-only text() is gone;
//   - the HUD's words stand alone, on no background, and the game calls notify nowhere: no system text, neither a
//     confirmation nor a refusal (show, don't tell); why a SPACE did nothing is kept for the View alone.
func TestNoMessageLine(t *testing.T) {
	files, _ := filepath.Glob("*.go")
	var all strings.Builder
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		src := string(b)
		all.WriteString(src)
		for _, bad := range []struct{ re, why string }{
			{`\bg\.message\b|^\s*message\s+string`, "a message field: say it with g.notify"},
			{`(?m)^func text\(|\btext\(s(creen)?,`, "text(): print through the HUD's notice, or printAt in a screen of its own"},
		} {
			if m := regexp.MustCompile("(?m)" + bad.re).FindString(src); m != "" {
				t.Errorf("%s: %q: %s", f, strings.TrimSpace(m), bad.why)
			}
		}
		if f != "hud.go" && regexp.MustCompile(`\bg\.note\.`).MatchString(src) {
			t.Errorf("%s touches g.note: only hud.go does; say it with g.notify, read it with g.said", f)
		}
		if f == "hud.go" && strings.Contains(src, "rect(") {
			t.Errorf("hud.go draws a rectangle: the HUD's words stand alone, never on a background")
		}
		if f != "hud.go" && strings.Contains(src, "g.notify(") {
			t.Errorf("%s says something in words: the game has no system text; show it in the world, on the counters or in the store's own page", f)
		}
		if f != "view.go" && strings.Contains(src, "g.refused") {
			t.Errorf("%s reads g.refused: it is the View's, set by g.refuse and never drawn", f)
		}
		if f != "draw.go" && strings.Contains(src, "DebugPrint") {
			t.Errorf("%s prints the debug font itself: only printAt does", f)
		}
	}
	src := all.String()
	a := strings.Index(src, "func (g *Game) drawFrame(")
	if a < 0 {
		t.Fatal("no drawFrame")
	}
	body := src[a:]
	body = body[:strings.Index(body, "\n}\n")]
	if m := regexp.MustCompile(`printAt\(|tinted(At)?\(|DebugPrint`).FindString(body); m != "" {
		t.Errorf("drawFrame prints text itself (%s): the HUD is the only place for a notification", m)
	}
	if last := strings.TrimSpace(body[strings.LastIndex(body, "\n"):]); !strings.HasPrefix(last, "g.drawHUD(screen)") {
		t.Errorf("drawFrame ends with %q: nothing is drawn after the HUD", last)
	}
}

// U2: a notice slides in when said, wraps inside the side margin, and goes when dropped or after its hold.
func TestNotice(t *testing.T) {
	g, _ := headless(t, 2, 1, 1)
	g.startRun()
	g.notify("CAN'T AFFORD")
	for range slideFrames {
		g.updateHUD()
	}
	if g.said() == "" || g.note.in != 1 {
		t.Fatalf("said %q, slid in %.2f; want the notice in", g.said(), g.note.in)
	}
	for _, l := range g.note.lines() {
		if len(l) > noticeWrap {
			t.Errorf("the line %q is over %d letters", l, noticeWrap)
		}
	}
	for range holdFrames + slideFrames {
		g.updateHUD()
	}
	if g.note.in != 0 || g.said() == "" {
		t.Fatalf("after its hold: slid in %.2f, said %q; want it out of sight, still what was last said", g.note.in, g.said())
	}
	if g.notify(""); g.said() != "" {
		t.Fatalf("dropped, it still says %q", g.said())
	}
}
