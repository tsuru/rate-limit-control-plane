package aggregator

import (
	"log/slog"
	"os"

	"github.com/tsuru/rate-limit-control-plane/internal/ratelimit"
)

// TODO: Manter os valores da ultima agregacao
// TODO: Utilizar os valores da ultima agregacao para calcular o delta
// TODO: Enviar o delta (calculado) para o pod worker
// Formula: EXCESS' = EXCESS + SOMATORIO(EXCESS - Pod.Excess)
// Se o EXCESS' for menor que 0, zerar o EXCESS' e manter o LAST
// Pegar o maior valor do LAST entre os pods
// TODO: Se o excess e o last forem iguais, ignorar o POD
func AggregateZones(zonePerPod []ratelimit.Zone, fullZone map[ratelimit.FullZoneKey]*ratelimit.RateLimitEntry) (ratelimit.Zone, map[ratelimit.FullZoneKey]*ratelimit.RateLimitEntry) {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		AddSource: true,
	}))
	logger.Info("Aggregating zones")
	logger.Info("Processing zones", "zones", zonePerPod)
	newFullZone := make(map[ratelimit.FullZoneKey]*ratelimit.RateLimitEntry)
	for _, podZone := range zonePerPod {
		for _, entity := range podZone.RateLimitEntries {
			logger.Info("Processing pod zone", "RateLimitHeader", podZone.RateLimitHeader)
			hashID := ratelimit.FullZoneKey{
				Zone: podZone.Name,
				Key:  entity.Key.String(podZone.RateLimitHeader),
			}

			oldEntry, oldExists := fullZone[hashID]
			if !oldExists {
				logger.Info("Old entry not found, creating new one", "hashID", hashID)
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

			logger.Info("Processing entity", "entity", entity)
			entry.Excess += entity.Excess - oldEntry.Excess
			entry.Last = max(entry.Last, entity.Last)
			logger.Info("Updated entry", "entry", entry)
		}
	}
	zone := ratelimit.Zone{
		Name:             zonePerPod[0].Name,
		RateLimitHeader:  zonePerPod[0].RateLimitHeader,
		RateLimitEntries: make([]ratelimit.RateLimitEntry, 0, len(newFullZone)),
	}
	logger.Info("Processing final value of aggregated excess")
	for key, entry := range newFullZone {
		logger.Info("Processing entry", "entry", entry, "key", key)
		oldEntry, oldExists := fullZone[key]
		logger.Info("Old entry status", "oldEntry", oldEntry, "oldExists", oldExists)
		if !oldExists {
			oldEntry = &ratelimit.RateLimitEntry{
				Key:    entry.Key,
				Last:   0,
				Excess: 0,
			}
		}
		logger.Info("Making last summation for entry", "entry", entry, "sum", entry.Excess+oldEntry.Excess)
		entry.Excess += oldEntry.Excess
		if entry.Excess < 0 {
			logger.Info("Entry excess is less than 0, setting to 0")
			entry.Excess = 0
		}
		zone.RateLimitEntries = append(zone.RateLimitEntries, *entry)
	}
	logger.Info("Finished aggregating zones")
	return zone, newFullZone
}
