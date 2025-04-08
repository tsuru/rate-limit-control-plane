package manager

import (
	"encoding/base64"
	"fmt"
	"log"
	"net"
	"sync"
	"time"

	"github.com/tsuru/rate-limit-control-plane/server"
)

// Update RpaasInstanceSyncWorker to use GoroutineManager for managing pod workers
type RpaasInstanceSyncWorker struct {
	sync.Mutex
	RpaasInstanceName    string
	Zones                []string
	RpaasInstanceSignals RpaasInstanceSignals
	PodWorkerManager     *GoroutineManager
	Ticker               *time.Ticker
	zoneDataChan         chan Optional[Zone]
	fullZone             map[FullZoneKey]*RateLimitEntry
	notify               chan server.Data
}

type RpaasInstanceSignals struct {
	StartRpaasPodWorker chan [2]string
	StopRpaasPodWorker  chan string
	StopChan            chan struct{}
}

func NewRpaasInstanceSyncWorker(rpaasInstanceName string, zones []string, notify chan server.Data) *RpaasInstanceSyncWorker {
	signals := RpaasInstanceSignals{
		StartRpaasPodWorker: make(chan [2]string),
		StopRpaasPodWorker:  make(chan string),
		StopChan:            make(chan struct{}),
	}
	ticker := time.NewTicker(10 * time.Second)

	return &RpaasInstanceSyncWorker{
		RpaasInstanceName:    rpaasInstanceName,
		Zones:                zones,
		RpaasInstanceSignals: signals,
		PodWorkerManager:     NewGoroutineManager(),
		Ticker:               ticker,
		zoneDataChan:         make(chan Optional[Zone]),
		fullZone:             make(map[FullZoneKey]*RateLimitEntry),
		notify:               notify,
	}
}

func (w *RpaasInstanceSyncWorker) Work() {
	for {
		select {
		case <-w.Ticker.C:
			w.processTick()
		case <-w.RpaasInstanceSignals.StopChan:
			w.cleanup()
			return
		}
	}
}

func (w *RpaasInstanceSyncWorker) processTick() {
	for _, zone := range w.Zones {
		w.Lock()
		podWorkers := w.PodWorkerManager.ListWorkerIDs()
		workerCount := len(podWorkers)

		if workerCount == 0 {
			w.Unlock()
			continue
		}

		// Process each zone for all pod workers
		w.PodWorkerManager.ForEachWorker(func(worker Worker) {
			if podWorker, ok := worker.(*RpaasPodWorker); ok {
				podWorker.ReadZoneChan <- zone
			}
		})

		// Collect zone data from all pod workers
		zoneData := []Zone{}
		for i := 0; i < workerCount; i++ {
			result := <-w.zoneDataChan
			if result.Error != nil {
				log.Printf("Error getting zone data: %v", result.Error)
				continue
			}
			zoneData = append(zoneData, result.Value)
		}

		if len(zoneData) == 0 {
			w.Unlock()
			continue
		}

		// Aggregate zone data
		aggregatedZone := w.aggregateZones(zoneData)
		w.Unlock()

		for _, entry := range aggregatedZone.RateLimitEntries {
			w.notify <- server.Data{
				ID:     net.IP(entry.Key).String(),
				Last:   entry.Last,
				Excess: entry.Excess,
			}
		}
		// Write aggregated data back to pod workers
		w.PodWorkerManager.ForEachWorker(func(worker Worker) {
			if podWorker, ok := worker.(*RpaasPodWorker); ok {
				podWorker.WriteZoneChan <- aggregatedZone
			}
		})
	}
}

func (w *RpaasInstanceSyncWorker) cleanup() {
	// Stop all pod workers
	w.PodWorkerManager.ForEachWorker(func(worker Worker) {
		worker.Stop()
	})

	// Close all channels
	close(w.RpaasInstanceSignals.StopChan)
	close(w.zoneDataChan)
	w.Ticker.Stop()
}

func (w *RpaasInstanceSyncWorker) Start() {
	go w.Work()
}

func (w *RpaasInstanceSyncWorker) Stop() {
	if w.RpaasInstanceSignals.StopChan != nil {
		w.RpaasInstanceSignals.StopChan <- struct{}{}
	}
}

func (w *RpaasInstanceSyncWorker) GetID() string {
	return w.RpaasInstanceName
}

func (w *RpaasInstanceSyncWorker) AddPodWorker(podIP, podName string) {
	podWorker := NewRpaasPodWorker(podIP, podName, w.zoneDataChan)
	w.PodWorkerManager.AddWorker(podWorker)
}

func (w *RpaasInstanceSyncWorker) aggregateZones(zonePerPod []Zone) Zone {
	newFullZone := make(map[FullZoneKey]*RateLimitEntry)
	for _, podZone := range zonePerPod {
		for _, entity := range podZone.RateLimitEntries {
			sEnc := base64.StdEncoding.EncodeToString([]byte(entity.Key))
			hashID := FullZoneKey{
				Zone: podZone.Name,
				Key:  sEnc,
			}
			// TODO: Maybe, for the newFullZone, we dont need to check if the entry exists. Check later
			if entry, exists := newFullZone[hashID]; exists {
				entry.Excess += entity.Excess
				// TODO: Use max value
				entry.Last = entity.Last
			} else {
				newFullZone[hashID] = &RateLimitEntry{
					Key:    entity.Key,
					Last:   entity.Last,
					Excess: entity.Excess,
				}
			}
		}
	}
	fmt.Printf("New Full Zone %+v\n", newFullZone)
	var zone Zone
	zone.Name = zonePerPod[0].Name
	zone.RateLimitEntries = make([]RateLimitEntry, 0, len(newFullZone))
	for _, entry := range newFullZone {
		fmt.Println("Accumulated Excess", entry.Excess)
		entry.Excess = entry.Excess / int64(len(zonePerPod))
		fmt.Println("Average Excess", entry.Excess)

		zone.RateLimitEntries = append(zone.RateLimitEntries, *entry)
	}
	w.fullZone = newFullZone
	return zone
}
