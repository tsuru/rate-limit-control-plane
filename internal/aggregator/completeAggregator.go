package aggregator

import (
	"github.com/tsuru/rate-limit-control-plane/internal/ratelimit"
)

type CompleteAggregator struct{}

func (a *CompleteAggregator) AggregateZones(zonePerPod []ratelimit.Zone, fullZone map[ratelimit.FullZoneKey]*ratelimit.RateLimitEntry) (ratelimit.Zone, map[ratelimit.FullZoneKey]*ratelimit.RateLimitEntry) {
	newFullZone := make(map[ratelimit.FullZoneKey]*ratelimit.RateLimitEntry)
	for _, podZone := range zonePerPod {
		for _, entity := range podZone.RateLimitEntries {
			hashID := ratelimit.FullZoneKey{
				Zone: podZone.Name,
				Key:  entity.Key.String(podZone.RateLimitHeader),
			}

			oldEntry, oldExists := fullZone[hashID]
			if !oldExists {
				oldEntry = &ratelimit.RateLimitEntry{
					Key:    entity.Key,
					Last:   0,
					Excess: 0,
				}
			}

			entry, exists := newFullZone[hashID]
			if !exists {
				entry = &ratelimit.RateLimitEntry{
					Key: entity.Key,
				}
				newFullZone[hashID] = entry
			}

			entry.Excess += entity.Excess - oldEntry.Excess
			entry.Last = max(entry.Last, entity.Last)
		}
	}
	zone := ratelimit.Zone{
		Name:             zonePerPod[0].Name,
		RateLimitHeader:  zonePerPod[0].RateLimitHeader,
		RateLimitEntries: make([]ratelimit.RateLimitEntry, 0, len(newFullZone)),
	}
	for key, entry := range newFullZone {
		oldEntry, oldExists := fullZone[key]
		if !oldExists {
			oldEntry = &ratelimit.RateLimitEntry{
				Key:    entry.Key,
				Last:   0,
				Excess: 0,
			}
		}
		entry.Excess += oldEntry.Excess
		if entry.Excess < 0 {
			entry.Excess = 0
		}
		zone.RateLimitEntries = append(zone.RateLimitEntries, *entry)
	}
	return zone, newFullZone
}
