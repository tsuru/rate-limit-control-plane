package repository

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"slices"
	"sync"

	"github.com/tsuru/rate-limit-control-plane/internal/logger"
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
	return &ZoneDataRepository{
		logger:                repositoryLogger,
		Data:                  make(map[string][]byte),
		readRpaasZoneDataChan: readRpaasZoneDataChan,
	}, readRpaasZoneDataChan
}

func (z *ZoneDataRepository) StartReader() {
	for rpaasZoneData := range z.readRpaasZoneDataChan {
		z.Lock()
		serverData := []Data{}
		for _, zone := range rpaasZoneData.Data {
			for _, entry := range zone.RateLimitEntries {
				serverData = append(serverData, Data{
					ID:     fmt.Sprintf("%s:%s", entry.Key.String(zone.RateLimitHeader), zone.Name),
					Last:   entry.Last,
					Excess: entry.Excess,
				})
			}
		}
		slices.SortFunc(serverData, func(a, b Data) int {
			if a.Excess < b.Excess {
				return 1
			}
			if a.Excess > b.Excess {
				return -1
			}
			return 0
		})
		if len(serverData) > 100 {
			serverData = serverData[:100]
		}
		dataBytes, err := json.MarshalIndent(serverData, "  ", "  ")
		if err != nil {
			z.logger.Error("Error marshaling JSON", "error", err)
			z.Unlock()
			continue
		}
		z.Data[rpaasZoneData.RpaasName] = dataBytes
		z.Unlock()
	}
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
