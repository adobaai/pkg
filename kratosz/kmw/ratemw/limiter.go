package ratemw

import (
	"github.com/maypok86/otter/v2"
	"golang.org/x/time/rate"
)

const (
	bytesPerLimiter  = 80
	bytesPerEntry    = 32
	bytesMapOverhead = 66
	bytesAvgUserID   = 36
	bytesPerRecord   = bytesPerLimiter + bytesPerEntry + bytesMapOverhead + bytesAvgUserID
)

type Store struct {
	cache *otter.Cache[string, *rate.Limiter]
	opts  Options
}

type KeyStrategy int

const (
	// StrategyAuth applies limits keyed by authenticated user identity.
	StrategyAuth KeyStrategy = iota
	// StrategyCookie applies limits keyed by anonymous rate-limit cookie token.
	StrategyCookie
	// StrategyIP applies limits keyed by client IP address.
	StrategyIP
)

type ResolvedKey struct {
	Key      string
	Strategy KeyStrategy
}

var defaultStrategyLimits = map[KeyStrategy]Options{
	StrategyAuth:   {Rate: 10, Burst: 8},
	StrategyCookie: {Rate: 3, Burst: 6},
	StrategyIP:     {Rate: 1, Burst: 5},
}

type StrategyStores struct {
	stores map[KeyStrategy]*Store
}

func NewStrategyStores(base Options) *StrategyStores {
	return NewStrategyStoresWithLimits(base, defaultStrategyLimits)
}

func NewStrategyStoresWithLimits(base Options, limits map[KeyStrategy]Options) *StrategyStores {
	if limits == nil {
		limits = defaultStrategyLimits
	}

	stores := make(map[KeyStrategy]*Store, len(limits))
	for strategy, opts := range limits {
		merged := base
		merged.Rate = opts.Rate
		merged.Burst = opts.Burst
		stores[strategy] = NewStore(merged)
	}

	if _, ok := stores[StrategyIP]; !ok {
		merged := base
		merged.Rate = defaultStrategyLimits[StrategyIP].Rate
		merged.Burst = defaultStrategyLimits[StrategyIP].Burst
		stores[StrategyIP] = NewStore(merged)
	}

	return &StrategyStores{stores: stores}
}

func (s *StrategyStores) Get(resolved ResolvedKey) *rate.Limiter {
	store, ok := s.stores[resolved.Strategy]
	if !ok {
		store = s.stores[StrategyIP]
	}
	return store.Get(resolved.Key)
}

func (s *StrategyStores) LimitFor(strategy KeyStrategy) Options {
	store, ok := s.stores[strategy]
	if ok {
		return store.opts
	}
	if fallback, ok := s.stores[StrategyIP]; ok {
		return fallback.opts
	}
	return defaultStrategyLimits[StrategyIP]
}

func (s *StrategyStores) Close() {
	for _, store := range s.stores {
		store.Close()
	}
}

func NewStore(opts Options) *Store {
	defaults := DefaultOptions()
	if opts.StaleAfter <= 0 {
		opts.StaleAfter = defaults.StaleAfter
	}
	if opts.MaxEntries <= 0 {
		opts.MaxEntries = defaults.MaxEntries
	}
	if opts.Burst < 0 {
		opts.Burst = 0
	}
	if opts.Rate < 0 {
		opts.Rate = 0
	}

	cache := otter.Must(&otter.Options[string, *rate.Limiter]{
		MaximumSize:      opts.MaxEntries,
		ExpiryCalculator: otter.ExpiryAccessing[string, *rate.Limiter](opts.StaleAfter),
	})

	return &Store{
		cache: cache,
		opts:  opts,
	}
}

func (s *Store) Get(userID string) *rate.Limiter {
	if limiter, ok := s.cache.GetIfPresent(userID); ok {
		return limiter
	}

	limiter, _ := s.cache.ComputeIfAbsent(userID, func() (*rate.Limiter, bool) {
		return rate.NewLimiter(s.opts.Rate, s.opts.Burst), false
	})
	return limiter
}

// Len returns the exact number of currently visible entries.
// It iterates over all entries, so it is O(n) and should be used for
// tests/debugging, not in hot paths.
func (s *Store) Len() int {
	count := 0
	for range s.cache.All() {
		count++
	}
	return count
}

func (s *Store) EstimateMemoryBytes() int {
	// EstimatedSize is O(1) and avoids scanning the whole cache.
	return s.cache.EstimatedSize() * bytesPerRecord
}

func (s *Store) EstimateMemoryMB() float64 {
	return float64(s.EstimateMemoryBytes()) / (1024 * 1024)
}

func (s *Store) Close() {
	s.cache.InvalidateAll()
	s.cache.StopAllGoroutines()
}
