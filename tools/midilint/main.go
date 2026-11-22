// midilint checks a score .mid: go run ./tools/midilint [file.mid]. With no file it lints the embedded
// sound/data/fugue.mid. Exits 1 on any `error:` line.
package main

import (
	"fmt"
	"os"
	"strings"

	"ghosthotel/sound"
)

func main() {
	var data []byte
	var err error
	if len(os.Args) > 1 {
		data, err = os.ReadFile(os.Args[1])
	} else {
		data, err = sound.Data.ReadFile("data/fugue.mid")
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	sc, names, err := sound.ParseTracks(data)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fail := false
	for _, l := range sound.Lint(sc, names) {
		fmt.Println(l)
		fail = fail || strings.HasPrefix(l, "error:")
	}
	for v, vn := range append(sound.VoiceNames[:], sound.RealConcierge) {
		ns := sc.Real
		if v < len(sc.Notes) {
			ns = sc.Notes[v]
		}
		if len(ns) == 0 {
			fmt.Printf("%-14s 0 notes\n", vn)
			continue
		}
		lo, hi := ns[0].Pitch, ns[0].Pitch
		for _, n := range ns {
			lo, hi = min(lo, n.Pitch), max(hi, n.Pitch)
		}
		fmt.Printf("%-14s %d notes, %s to %s\n", vn, len(ns), sound.NoteName(lo), sound.NoteName(hi))
	}
	fmt.Printf("coda at beat %g, end at beat %g\n", sc.Coda, sc.End)
	if fail {
		os.Exit(1)
	}
}
