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
	mu    sync.Mutex
	tasks map[string]Params
}

type workFunc func(zone string) (Zone, error)
