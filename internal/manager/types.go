package manager

type Optional[T any] struct {
	Value T
	Error error
}

type Worker interface {
	Start()
	Stop()
	GetID() string
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
