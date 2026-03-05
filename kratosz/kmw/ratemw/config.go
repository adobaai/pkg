package ratemw

import (
	"time"

	"golang.org/x/time/rate"
)

type Options struct {
	Rate       rate.Limit
	Burst      int
	StaleAfter time.Duration
	MaxEntries int
}

func DefaultOptions() Options {
	return Options{
		Rate:       rate.Limit(1),
		Burst:      10,
		StaleAfter: 10 * time.Minute,
		MaxEntries: 100_000,
	}
}
