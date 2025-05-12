package aggregator

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/tsuru/rate-limit-control-plane/internal/ratelimit"
)

func TestAggregateZones(t *testing.T) {
	t.Run("Should aggregate zone without previous data", func(t *testing.T) {
		zonePerPod := []ratelimit.Zone{
			{
				Name: "zone1",
				RateLimitHeader: ratelimit.RateLimitHeader{
					Key:          ratelimit.RemoteAddress,
					Now:          400,
					NowMonotonic: 40,
				},
				RateLimitEntries: []ratelimit.RateLimitEntry{
					{Key: []byte("key1"), Last: 10, Excess: 5},
					{Key: []byte("key2"), Last: 20, Excess: 10},
				},
			},
			{
				Name: "zone1",
				RateLimitHeader: ratelimit.RateLimitHeader{
					Key:          ratelimit.RemoteAddress,
					Now:          400,
					NowMonotonic: 40,
				},
				RateLimitEntries: []ratelimit.RateLimitEntry{
					{Key: []byte("key1"), Last: 15, Excess: 7},
					{Key: []byte("key3"), Last: 25, Excess: 12},
				},
			},
		}
		fullZone := map[ratelimit.FullZoneKey]*ratelimit.RateLimitEntry{}

		expectedZone := ratelimit.Zone{
			Name: "zone1",
			RateLimitHeader: ratelimit.RateLimitHeader{
				Key:          ratelimit.RemoteAddress,
				Now:          400,
				NowMonotonic: 40,
			},
			RateLimitEntries: []ratelimit.RateLimitEntry{
				{
					Key:    []byte("key1"),
					Last:   15,
					Excess: 12,
				},
				{
					Key:    []byte("key2"),
					Last:   20,
					Excess: 10,
				},
				{
					Key:    []byte("key3"),
					Last:   25,
					Excess: 12,
				},
			},
		}

		expectedFullZone := map[ratelimit.FullZoneKey]*ratelimit.RateLimitEntry{
			{Zone: "zone1", Key: "key1"}: {Key: []byte("key1"), Last: 15, Excess: 12},
			{Zone: "zone1", Key: "key2"}: {Key: []byte("key2"), Last: 20, Excess: 10},
			{Zone: "zone1", Key: "key3"}: {Key: []byte("key3"), Last: 25, Excess: 12},
		}

		resultZone, resultFullZone := AggregateZones(zonePerPod, fullZone)

		assertZone(t, expectedZone, resultZone)
		assert.Equal(t, expectedFullZone, resultFullZone)
	})

	t.Run("Should aggregate zone with previous data", func(t *testing.T) {
		zonePerPod := []ratelimit.Zone{
			{
				Name: "zone1",
				RateLimitHeader: ratelimit.RateLimitHeader{
					Key:          ratelimit.RemoteAddress,
					Now:          400,
					NowMonotonic: 40,
				},
				RateLimitEntries: []ratelimit.RateLimitEntry{
					{Key: []byte("key1"), Last: 20, Excess: 15},
					{Key: []byte("key2"), Last: 40, Excess: 15},
				},
			},
			{
				Name: "zone1",
				RateLimitHeader: ratelimit.RateLimitHeader{
					Key:          ratelimit.RemoteAddress,
					Now:          400,
					NowMonotonic: 40,
				},
				RateLimitEntries: []ratelimit.RateLimitEntry{
					{Key: []byte("key1"), Last: 30, Excess: 11},
					{Key: []byte("key3"), Last: 50, Excess: 17},
				},
			},
		}

		fullZone := map[ratelimit.FullZoneKey]*ratelimit.RateLimitEntry{
			{Zone: "zone1", Key: "key1"}: {Key: []byte("key1"), Last: 15, Excess: 12},
			{Zone: "zone1", Key: "key2"}: {Key: []byte("key2"), Last: 20, Excess: 10},
			{Zone: "zone1", Key: "key3"}: {Key: []byte("key3"), Last: 25, Excess: 12},
		}

		expectedZone := ratelimit.Zone{
			Name: "zone1",
			RateLimitHeader: ratelimit.RateLimitHeader{
				Key:          ratelimit.RemoteAddress,
				Now:          400,
				NowMonotonic: 40,
			},
			RateLimitEntries: []ratelimit.RateLimitEntry{
				{
					Key:    []byte("key1"),
					Last:   30,
					Excess: 14,
				},
				{
					Key:    []byte("key2"),
					Last:   40,
					Excess: 15,
				},
				{
					Key:    []byte("key3"),
					Last:   50,
					Excess: 17,
				},
			},
		}

		expectedFullZone := map[ratelimit.FullZoneKey]*ratelimit.RateLimitEntry{
			{Zone: "zone1", Key: "key1"}: {Key: []byte("key1"), Last: 30, Excess: 14},
			{Zone: "zone1", Key: "key2"}: {Key: []byte("key2"), Last: 40, Excess: 15},
			{Zone: "zone1", Key: "key3"}: {Key: []byte("key3"), Last: 50, Excess: 17},
		}

		resultZone, resultFullZone := AggregateZones(zonePerPod, fullZone)

		assertZone(t, expectedZone, resultZone)
		assert.Equal(t, expectedFullZone, resultFullZone)
	})

	t.Run("consecutive runs with update", func(t *testing.T) {
		zonePerPod := []ratelimit.Zone{
			{
				Name: "zone1",
				RateLimitHeader: ratelimit.RateLimitHeader{
					Key:          ratelimit.RemoteAddress,
					Now:          400,
					NowMonotonic: 40,
				},
				RateLimitEntries: []ratelimit.RateLimitEntry{
					{Key: []byte("key1"), Last: 10, Excess: 5},
					{Key: []byte("key2"), Last: 20, Excess: 10},
				},
			},
			{
				Name: "zone1",
				RateLimitHeader: ratelimit.RateLimitHeader{
					Key:          ratelimit.RemoteAddress,
					Now:          400,
					NowMonotonic: 40,
				},
				RateLimitEntries: []ratelimit.RateLimitEntry{
					{Key: []byte("key1"), Last: 15, Excess: 7},
					{Key: []byte("key3"), Last: 25, Excess: 12},
				},
			},
		}
		fullZone := map[ratelimit.FullZoneKey]*ratelimit.RateLimitEntry{}

		expectedZone := ratelimit.Zone{
			Name: "zone1",
			RateLimitHeader: ratelimit.RateLimitHeader{
				Key:          ratelimit.RemoteAddress,
				Now:          400,
				NowMonotonic: 40,
			},
			RateLimitEntries: []ratelimit.RateLimitEntry{
				{
					Key:    []byte("key1"),
					Last:   15,
					Excess: 12,
				},
				{
					Key:    []byte("key2"),
					Last:   20,
					Excess: 10,
				},
				{
					Key:    []byte("key3"),
					Last:   25,
					Excess: 12,
				},
			},
		}

		expectedFullZone := map[ratelimit.FullZoneKey]*ratelimit.RateLimitEntry{
			{Zone: "zone1", Key: "key1"}: {Key: []byte("key1"), Last: 15, Excess: 12},
			{Zone: "zone1", Key: "key2"}: {Key: []byte("key2"), Last: 20, Excess: 10},
			{Zone: "zone1", Key: "key3"}: {Key: []byte("key3"), Last: 25, Excess: 12},
		}

		resultZone, resultFullZone := AggregateZones(zonePerPod, fullZone)

		assertZone(t, expectedZone, resultZone)
		assert.Equal(t, expectedFullZone, resultFullZone)

		secondZonePerPod := []ratelimit.Zone{
			{
				Name: "zone1",
				RateLimitHeader: ratelimit.RateLimitHeader{
					Key:          ratelimit.RemoteAddress,
					Now:          405,
					NowMonotonic: 45,
				},
				RateLimitEntries: []ratelimit.RateLimitEntry{
					{Key: []byte("key1"), Last: 15, Excess: 12},
					{Key: []byte("key2"), Last: 20, Excess: 10},
					{Key: []byte("key3"), Last: 25, Excess: 12},
				},
			},
			{
				Name: "zone1",
				RateLimitHeader: ratelimit.RateLimitHeader{
					Key:          ratelimit.RemoteAddress,
					Now:          405,
					NowMonotonic: 45,
				},
				RateLimitEntries: []ratelimit.RateLimitEntry{
					{Key: []byte("key1"), Last: 15, Excess: 12},
					{Key: []byte("key2"), Last: 20, Excess: 10},
					{Key: []byte("key3"), Last: 25, Excess: 12},
				},
			},
		}

		secondExpectedZone := ratelimit.Zone{
			Name: "zone1",
			RateLimitHeader: ratelimit.RateLimitHeader{
				Key:          ratelimit.RemoteAddress,
				Now:          405,
				NowMonotonic: 45,
			},
			RateLimitEntries: []ratelimit.RateLimitEntry{
				{
					Key:    []byte("key1"),
					Last:   15,
					Excess: 12,
				},
				{
					Key:    []byte("key2"),
					Last:   20,
					Excess: 10,
				},
				{
					Key:    []byte("key3"),
					Last:   25,
					Excess: 12,
				},
			},
		}

		secondExpectedFullZone := map[ratelimit.FullZoneKey]*ratelimit.RateLimitEntry{
			{Zone: "zone1", Key: "key1"}: {Key: []byte("key1"), Last: 15, Excess: 12},
			{Zone: "zone1", Key: "key2"}: {Key: []byte("key2"), Last: 20, Excess: 10},
			{Zone: "zone1", Key: "key3"}: {Key: []byte("key3"), Last: 25, Excess: 12},
		}

		secondResultZone, secondResultFullZone := AggregateZones(secondZonePerPod, resultFullZone)

		assertZone(t, secondExpectedZone, secondResultZone)
		assert.Equal(t, secondExpectedFullZone, secondResultFullZone)
	})

	t.Run("Should set excess to 0 if negative", func(t *testing.T) {
		zonePerPod := []ratelimit.Zone{
			{
				Name: "zone1",
				RateLimitHeader: ratelimit.RateLimitHeader{
					Key:          ratelimit.RemoteAddress,
					Now:          400,
					NowMonotonic: 40,
				},
				RateLimitEntries: []ratelimit.RateLimitEntry{
					{Key: []byte("key1"), Last: 20, Excess: 15},
					{Key: []byte("key2"), Last: 40, Excess: 15},
					{Key: []byte("key3"), Last: 65, Excess: 2},
				},
			},
			{
				Name: "zone1",
				RateLimitHeader: ratelimit.RateLimitHeader{
					Key:          ratelimit.RemoteAddress,
					Now:          400,
					NowMonotonic: 40,
				},
				RateLimitEntries: []ratelimit.RateLimitEntry{
					{Key: []byte("key1"), Last: 30, Excess: 11},
					{Key: []byte("key3"), Last: 50, Excess: 2},
				},
			},
		}

		fullZone := map[ratelimit.FullZoneKey]*ratelimit.RateLimitEntry{
			{Zone: "zone1", Key: "key1"}: {Key: []byte("key1"), Last: 15, Excess: 12},
			{Zone: "zone1", Key: "key2"}: {Key: []byte("key2"), Last: 20, Excess: 10},
			{Zone: "zone1", Key: "key3"}: {Key: []byte("key3"), Last: 25, Excess: 12},
		}

		expectedZone := ratelimit.Zone{
			Name: "zone1",
			RateLimitHeader: ratelimit.RateLimitHeader{
				Key:          ratelimit.RemoteAddress,
				Now:          400,
				NowMonotonic: 40,
			},
			RateLimitEntries: []ratelimit.RateLimitEntry{
				{
					Key:    []byte("key1"),
					Last:   30,
					Excess: 14,
				},
				{
					Key:    []byte("key2"),
					Last:   40,
					Excess: 15,
				},
				{
					Key:    []byte("key3"),
					Last:   65,
					Excess: 0,
				},
			},
		}

		expectedFullZone := map[ratelimit.FullZoneKey]*ratelimit.RateLimitEntry{
			{Zone: "zone1", Key: "key1"}: {Key: []byte("key1"), Last: 30, Excess: 14},
			{Zone: "zone1", Key: "key2"}: {Key: []byte("key2"), Last: 40, Excess: 15},
			{Zone: "zone1", Key: "key3"}: {Key: []byte("key3"), Last: 65, Excess: 0},
		}

		resultZone, resultFullZone := AggregateZones(zonePerPod, fullZone)

		assertZone(t, expectedZone, resultZone)
		assert.Equal(t, expectedFullZone, resultFullZone)
	})
}

func assertZone(t *testing.T, expected, actual ratelimit.Zone) {
	t.Helper()
	assert.Equal(t, expected.Name, actual.Name)
	assert.Equal(t, expected.RateLimitHeader, actual.RateLimitHeader)
	assert.Equal(t, len(expected.RateLimitEntries), len(actual.RateLimitEntries))
	// Compare ignoring order
	assert.ElementsMatch(t, expected.RateLimitEntries, actual.RateLimitEntries)
}
