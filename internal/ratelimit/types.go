package ratelimit

import (
	"net"
)

const (
	BinaryRemoteAddress = "$binary_remote_addr"
	RemoteAddress       = "$remote_addr"
)

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

func (r *RateLimitEntry) NonMonotic(header RateLimitHeader) {
	r.Last = header.Now - (header.NowMonotonic - r.Last)
}

func (r *RateLimitEntry) Monotonic(header RateLimitHeader) {
	r.Last = header.NowMonotonic - (header.Now - r.Last)
}

type Key []byte

func (r Key) String(header RateLimitHeader) string {
	switch header.Key {
	case BinaryRemoteAddress:
		return net.IP(r).String()
	case RemoteAddress:
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
