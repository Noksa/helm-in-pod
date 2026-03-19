package logz

import (
	"sync"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

var (
	hostOnce    sync.Once
	podOnce     sync.Once
	hostPodOnce sync.Once

	hostLogger    zerolog.Logger
	podLogger     zerolog.Logger
	hostPodLogger zerolog.Logger
)

// Host returns a reusable zerolog.Logger with source=host field.
func Host() *zerolog.Logger {
	hostOnce.Do(func() {
		hostLogger = log.With().Str("source", "host").Logger()
	})
	return &hostLogger
}

// Pod returns a reusable zerolog.Logger with source=pod field.
func Pod() *zerolog.Logger {
	podOnce.Do(func() {
		podLogger = log.With().Str("source", "pod").Logger()
	})
	return &podLogger
}

// HostPod returns a reusable zerolog.Logger with source=host+pod field.
func HostPod() *zerolog.Logger {
	hostPodOnce.Do(func() {
		hostPodLogger = log.With().Str("source", "host+pod").Logger()
	})
	return &hostPodLogger
}
