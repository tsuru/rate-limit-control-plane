package repository

import (
	"encoding/json"
	"fmt"
	"log"
	"sync"

	"github.com/tsuru/rate-limit-control-plane/internal/manager"
)

type ZoneDataRepository struct {
	sync.Mutex
	// -> RPAAS-NAME'{id: IP-ZONE, Last, Excess}'
	Data                  map[string][]byte
	readRpaasZoneDataChan chan manager.RpaasZoneData
}

func NewRpaasZoneDataRepository() (*ZoneDataRepository, chan manager.RpaasZoneData) {
	readRpaasZoneDataChan := make(chan manager.RpaasZoneData)
	return &ZoneDataRepository{
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
				fmt.Println("StartReader: podZone.RateLimitHeader", zone.RateLimitHeader)
				serverData = append(serverData, Data{
					ID:     fmt.Sprintf("%s:%s", entry.Key.String(zone.RateLimitHeader), zone.Name),
					Last:   entry.Last,
					Excess: entry.Excess,
				})
			}
		}
		dataBytes, err := json.MarshalIndent(serverData, "  ", "  ")
		if err != nil {
			log.Println("Error marshaling JSON:", err)
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
