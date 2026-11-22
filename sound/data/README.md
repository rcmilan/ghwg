# sound/data

Embedded into the game (`sound.Data`). Every file is mono Ogg Vorbis at 22050 Hz (the mixer's `Rate`; other rates are resampled at load).

| File | What |
|---|---|
| `<instrument>_<midi>.ogg` | One note of an instrument, its pitch the MIDI note number (`sax_62.ogg` is a D4). Instruments: `bass`, `basswet` (the bass with long reverb: the Concierge in ghost), `sax`, `vibes`, `celesta`, `trumpet`, `whistle`. About one file per octave of each voice's range. |
| `step_<n>.ogg` | A footstep, `n` from 1. |

Sources and licenses: the README's Assets section.
