package sound

import (
	"time"

	"github.com/hajimehoshi/ebiten/v2/audio"
)

// Open starts the one audio context and the one player that streams m. Only a person's game calls it (main),
// never the headless sim, tests or -shot. In a browser the sound starts on the first key press: Ebitengine
// resumes the audio context on a user gesture.
func Open(m *Mixer) error {
	p, err := audio.NewContext(Rate).NewPlayerF32(m)
	if err != nil {
		return err
	}
	p.SetBufferSize(100 * time.Millisecond) // latency against underruns in wasm; notes land on frames anyway
	p.Play()
	return nil
}
