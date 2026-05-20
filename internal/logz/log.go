package logz

import (
	"sync"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

// Host returns a reusable zerolog.Logger with source=host field.
var Host = sync.OnceValue(func() *zerolog.Logger {
	l := log.With().Str("source", "host").Logger()
	return &l
})

// Pod returns a reusable zerolog.Logger with source=pod field.
var Pod = sync.OnceValue(func() *zerolog.Logger {
	l := log.With().Str("source", "pod").Logger()
	return &l
})

// HostPod returns a reusable zerolog.Logger with source=host+pod field.
var HostPod = sync.OnceValue(func() *zerolog.Logger {
	l := log.With().Str("source", "host+pod").Logger()
	return &l
})

// Suppress disables all log output globally. Used after a signal interrupt to
// silence goroutines that are still racing to finish their current log line.
func Suppress() {
	zerolog.SetGlobalLevel(zerolog.Disabled)
}
