# sound/data

Embedded into the game (`sound.Data`). Every file is mono Ogg Vorbis at 22050 Hz (the mixer's `Rate`; other rates are resampled at load).

| File | What |
|---|---|
| `fugue.mid` | The score: a Type-1 MIDI file, one track per voice named `Concierge`, `Magenta`, `Magda`, `Celestine`, `Tim`, `Osvaldo`, plus `Concierge real` (his line in the real dimension; without it `Concierge` plays in both); the 20-bar loop at 60 BPM, then a `coda` marker and the coda (the finale's music). |
| `<instrument>_<midi>.ogg` | One note of an instrument, its pitch the MIDI note number (`sax_62.ogg` is a D4). Instruments: `bass`, `basswet` (the bass with long reverb: the Concierge in ghost), `sax`, `vibes`, `celesta`, `trumpet`, `whistle`. About one file per octave of each voice's range. |
| `step_<n>.ogg` | A footstep, `n` from 1. |

Sources and licenses: the README's Assets section.
