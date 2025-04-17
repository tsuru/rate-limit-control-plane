package aggregator

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/tsuru/rate-limit-control-plane/internal/ratelimit"
)

func TestAggregateZones(t *testing.T) {
	zonePerPod := []ratelimit.Zone{
		{
			Name: "zone1",
			RateLimitHeader: ratelimit.RateLimitHeader{
				Key: ratelimit.RemoteAddress,
			},
			RateLimitEntries: []ratelimit.RateLimitEntry{
				{Key: []byte("key1"), Last: 10, Excess: 5},
				{Key: []byte("key2"), Last: 20, Excess: 10},
			},
		},
		{
			Name: "zone1",
			RateLimitHeader: ratelimit.RateLimitHeader{
				Key: ratelimit.RemoteAddress,
			},
			RateLimitEntries: []ratelimit.RateLimitEntry{
				{Key: []byte("key1"), Last: 15, Excess: 7},
				{Key: []byte("key3"), Last: 25, Excess: 12},
			},
		},
	}

	fullZone := map[ratelimit.FullZoneKey]*ratelimit.RateLimitEntry{
		{Zone: "zone1", Key: "key1"}: {Key: []byte("key1"), Last: 5, Excess: 3},
		{Zone: "zone1", Key: "key2"}: {Key: []byte("key2"), Last: 10, Excess: 8},
	}

	expectedZone := ratelimit.Zone{
		Name: "zone1",
		RateLimitHeader: ratelimit.RateLimitHeader{
			Key: ratelimit.RemoteAddress,
		},
		RateLimitEntries: []ratelimit.RateLimitEntry{
			{Key: []byte("key1"), Last: 15, Excess: 9},
			{Key: []byte("key2"), Last: 20, Excess: 10},
			{Key: []byte("key3"), Last: 25, Excess: 12},
		},
	}

	expectedFullZone := map[ratelimit.FullZoneKey]*ratelimit.RateLimitEntry{
		{Zone: "zone1", Key: "key1"}: {Key: []byte("key1"), Last: 15, Excess: 9},
		{Zone: "zone1", Key: "key2"}: {Key: []byte("key2"), Last: 20, Excess: 10},
		{Zone: "zone1", Key: "key3"}: {Key: []byte("key3"), Last: 25, Excess: 12},
	}

	resultZone, resultFullZone := AggregateZones(zonePerPod, fullZone)

	assert.Equal(t, expectedZone, resultZone)
	assert.Equal(t, expectedFullZone, resultFullZone)
}
