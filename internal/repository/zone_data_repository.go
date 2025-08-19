package repository

import (
	"encoding/json"
	"log/slog"
	"os"
	"runtime"
	"sync"

	"github.com/tsuru/rate-limit-control-plane/internal/config"
	"github.com/tsuru/rate-limit-control-plane/internal/logger"
	"github.com/tsuru/rate-limit-control-plane/internal/manager"
	"github.com/tsuru/rate-limit-control-plane/internal/ratelimit"
)

type ZoneDataRepository struct {
	sync.Mutex
	logger                *slog.Logger
	Data                  map[string][]byte
	readRpaasZoneDataChan chan ratelimit.RpaasZoneData
}

func NewRpaasZoneDataRepository() (*ZoneDataRepository, chan ratelimit.RpaasZoneData) {
	readRpaasZoneDataChan := make(chan ratelimit.RpaasZoneData)
	repositoryLogger := logger.NewLogger(map[string]string{"emitter": "rate-limit-control-plane-repository"}, os.Stdout)
	zoneRepository := &ZoneDataRepository{
		logger:                repositoryLogger,
		Data:                  make(map[string][]byte),
		readRpaasZoneDataChan: readRpaasZoneDataChan,
	}
	go zoneRepository.startReader()

	return zoneRepository, readRpaasZoneDataChan
}

func (z *ZoneDataRepository) startReader() {
	for rpaasZoneData := range z.readRpaasZoneDataChan {
		z.insert(rpaasZoneData)
	}
}

func (z *ZoneDataRepository) insert(rpaasZoneData ratelimit.RpaasZoneData) {
	z.Lock()
	defer z.Unlock()

	serverData := []Data{}
	for _, zone := range rpaasZoneData.Data {
		for _, entry := range zone.RateLimitEntries {
			serverData = append(serverData, Data{
				Key:    entry.Key.String(zone.RateLimitHeader),
				Zone:   zone.Name,
				Last:   entry.Last,
				Excess: entry.Excess,
			})
		}
	}
	serverData = TopKByExcess(serverData, config.Spec.MaxTopOffendersReport)
	dataBytes, err := json.MarshalIndent(serverData, "  ", "  ")
	if err != nil {
		z.logger.Error("Error marshaling JSON", "error", err)
		return
	}
	z.Data[rpaasZoneData.RpaasName] = dataBytes

	// Update repository memory usage metric
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)
	manager.GetZoneDataRepositoryMemoryGauge().Set(float64(memStats.HeapInuse))
}

func (z *ZoneDataRepository) GetRpaasZoneData(rpaasName string) ([]byte, bool) {
	z.Lock()
	defer z.Unlock()
	dataBytes, exists := z.Data[rpaasName]
	return dataBytes, exists
}

func (z *ZoneDataRepository) ListInstances() []string {
	z.Lock()
	defer z.Unlock()
	instance := make([]string, 0, len(z.Data))
	for key := range z.Data {
		instance = append(instance, key)
	}
	return instance
}
