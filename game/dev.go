//go:build !prod

package game

// Dev: a dev build (the default). The production build (-tags prod, prod.go) drops the dev flags, ?args=,
// story mode and the FPS readout.
const Dev = true
