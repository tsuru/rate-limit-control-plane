package manager

import "sync"

type Optional[T any] struct {
	Value T
	Error error
}

type Zone struct {
	Name             string
	RateLimitHeader  RateLimitHeader
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
	mu    sync.Mutex
	tasks map[string]*RpaasInstanceSyncWorker
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
