package sound

import (
	"encoding/binary"
	"errors"
	"fmt"
	"sort"
	"strings"
)

// Parse reads a Type-0 or Type-1 standard MIDI file into a Score (T399 MIDI): tracks named by VoiceNames, note
// on/off to Notes in beats (ticks over the file's division, a beat is a quarter), the `coda` marker to Coda;
// tempo, meter, program, controller and bend events ignored. Tracks with no name or an unknown one are skipped
// (Lint reports them through ParseTracks).
func Parse(data []byte) (*Score, error) {
	sc, _, err := ParseTracks(data)
	return sc, err
}

// ParseTracks is Parse plus every track name the file holds (trimmed, as written), for Lint.
func ParseTracks(data []byte) (*Score, []string, error) {
	if len(data) < 14 || string(data[:4]) != "MThd" || binary.BigEndian.Uint32(data[4:8]) < 6 {
		return nil, nil, errors.New("sound: not a MIDI file")
	}
	hl := int(binary.BigEndian.Uint32(data[4:8]))
	format := binary.BigEndian.Uint16(data[8:])
	div := binary.BigEndian.Uint16(data[12:])
	if format > 1 {
		return nil, nil, fmt.Errorf("sound: MIDI format %d not supported (want 0 or 1)", format)
	}
	if div == 0 || div&0x8000 != 0 {
		return nil, nil, errors.New("sound: SMPTE or zero MIDI division not supported")
	}
	if 8+hl > len(data) {
		return nil, nil, errors.New("sound: truncated MIDI header")
	}
	sc := &Score{Coda: -1}
	var names []string
	rest := data[8+hl:]
	for len(rest) >= 8 {
		n := int(binary.BigEndian.Uint32(rest[4:8]))
		if n > len(rest)-8 {
			return nil, nil, errors.New("sound: truncated MIDI chunk")
		}
		if string(rest[:4]) == "MTrk" {
			if err := parseTrack(rest[8:8+n], float64(div), sc, &names); err != nil {
				return nil, nil, err
			}
		}
		rest = rest[8+n:]
	}
	for _, ns := range append(sc.Notes[:], sc.Real) {
		sort.SliceStable(ns, func(i, j int) bool { return ns[i].Start < ns[j].Start })
		for _, n := range ns {
			sc.End = max(sc.End, n.Start+n.Len)
		}
	}
	if sc.Coda < 0 {
		sc.Coda = sc.End
	} else {
		sc.End = max(sc.End, sc.Coda)
	}
	return sc, names, nil
}

type pending struct{ ch, pitch, tick, vel int }

func parseTrack(b []byte, div float64, sc *Score, names *[]string) error {
	var (
		tick, status int
		name         string
		named        bool
		open         []pending // in note-on order, so the oldest same-pitch note closes first (FIFO)
		notes        []Note
	)
	bad := errors.New("sound: truncated MIDI track")
	vlq := func() (int, bool) {
		v := 0
		for i := 0; i < 4 && len(b) > 0; i++ {
			c := b[0]
			b = b[1:]
			v = v<<7 | int(c&0x7f)
			if c&0x80 == 0 {
				return v, true
			}
		}
		return 0, false
	}
	closeNote := func(ch, pitch int) {
		for i, p := range open {
			if p.ch == ch && p.pitch == pitch {
				notes = append(notes, Note{Pitch: pitch, Start: float64(p.tick) / div,
					Len: float64(tick-p.tick) / div, Vel: float32(p.vel) / 127})
				open = append(open[:i], open[i+1:]...)
				return
			}
		}
	}
	for len(b) > 0 {
		d, ok := vlq()
		if !ok || len(b) == 0 {
			return bad
		}
		tick += d
		if b[0] >= 0x80 {
			status = int(b[0])
			b = b[1:]
		} else if status == 0 {
			return errors.New("sound: MIDI data byte with no running status")
		}
		switch {
		case status == 0xFF:
			if len(b) < 1 {
				return bad
			}
			typ := b[0]
			b = b[1:]
			l, ok := vlq()
			if !ok || l > len(b) {
				return bad
			}
			text := strings.TrimSpace(string(b[:l]))
			b = b[l:]
			switch typ {
			case 0x03:
				if !named {
					name, named = text, true
				}
			case 0x01, 0x06, 0x07:
				if strings.EqualFold(text, "coda") && sc.Coda < 0 {
					sc.Coda = float64(tick) / div
				}
			}
			status = 0 // meta and sysex cancel running status
		case status == 0xF0 || status == 0xF7:
			l, ok := vlq()
			if !ok || l > len(b) {
				return bad
			}
			b = b[l:]
			status = 0
		default:
			args := 2
			if hi := status & 0xF0; hi == 0xC0 || hi == 0xD0 {
				args = 1
			}
			if len(b) < args {
				return bad
			}
			ch, p, v := status&0x0F, int(b[0]), 0
			if args == 2 {
				v = int(b[1])
			}
			b = b[args:]
			switch status & 0xF0 {
			case 0x90:
				if v > 0 {
					open = append(open, pending{ch, p, tick, v})
				} else {
					closeNote(ch, p)
				}
			case 0x80:
				closeNote(ch, p)
			}
		}
	}
	for len(open) > 0 { // stuck notes end with the track
		closeNote(open[0].ch, open[0].pitch)
	}
	if named && len(notes) > 0 { // a track with no notes (the conductor track, titled after the song) is no voice
		*names = append(*names, name)
		for v, vn := range VoiceNames {
			if strings.EqualFold(name, vn) {
				sc.Notes[v] = append(sc.Notes[v], notes...)
			}
		}
		if strings.EqualFold(name, RealConcierge) {
			sc.Real = append(sc.Real, notes...)
		}
	}
	return nil
}
