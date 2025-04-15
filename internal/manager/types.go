package manager

import (
	"net"
)

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

type RateLimitHeader struct {
	Key          string
	Now          int64
	NowMonotonic int64
}

type RateLimitEntry struct {
	Key    Key
	Last   int64
	Excess int64
}

type Key []byte

func (r Key) String(header RateLimitHeader) string {
	switch header.Key {
	case binaryRemoteAddress:
		return net.IP(r).String()
	case remoteAddress:
		fallthrough
	default:
		return string(r)
	}
}

type FullZoneKey struct {
	Zone string
	Key  string
}

type RpaasZoneData struct {
	RpaasName string
	Data      []Zone
}
