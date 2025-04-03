package manager

import "sync"

type Zone struct {
	Name             string
	RateLimitEntries []RateLimitEntry
}

type RateLimitEntry struct {
	Key    []byte
	Last   int64
	Excess int64
}

type Params struct {
	stop  chan bool
	work  chan bool
	zones []Zone
}

type GoroutineManager struct {
	mu       sync.Mutex
	tasks    map[string]Params
	fullZone map[FullZoneKey]RateLimitEntry
}

type FullZoneKey struct {
	Zone string
	Key  string
}

type workFunc func(zone string) (Zone, error)

type RateLimitHeader struct {
	Key          string
	Now          int64
	NowMonotonic int64
}
