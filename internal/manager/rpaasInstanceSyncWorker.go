package manager

import (
	"fmt"
	"log"
	"sync"
	"time"
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
	notify               chan RpaasZoneData
}

type RpaasInstanceSignals struct {
	StartRpaasPodWorker chan [2]string
	StopRpaasPodWorker  chan string
	StopChan            chan struct{}
}

func NewRpaasInstanceSyncWorker(rpaasInstanceName string, zones []string, notify chan RpaasZoneData) *RpaasInstanceSyncWorker {
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
	rpaasZoneData := RpaasZoneData{
		RpaasName: w.RpaasInstanceName,
		Data:      []Zone{},
	}
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

		rpaasZoneData.Data = append(rpaasZoneData.Data, aggregatedZone)

		// Write aggregated data back to pod workers
		w.PodWorkerManager.ForEachWorker(func(worker Worker) {
			if podWorker, ok := worker.(*RpaasPodWorker); ok {
				podWorker.WriteZoneChan <- aggregatedZone
			}
		})
	}
	w.notify <- rpaasZoneData
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

func (w *RpaasInstanceSyncWorker) RemovePodWorker(podName string) error {
	if ok := w.PodWorkerManager.RemoveWorker(podName); !ok {
		return fmt.Errorf("pod worker not found: %s", podName)
	}
	return nil
}

// TODO: Manter os valores da ultima agregacao
// TODO: Utilizar os valores da ultima agregacao para calcular o delta
// TODO: Enviar o delta (calculado) para o pod worker
// Formula: EXCESS' = EXCESS + SOMATORIO(EXCESS - Pod.Excess)
// Se o EXCESS' for menor que 0, zerar o EXCESS' e manter o LAST
// Pegar o maior valor do LAST entre os pods
// TODO: Se o excess e o last forem iguais, ignorar o POD
func (w *RpaasInstanceSyncWorker) aggregateZones(zonePerPod []Zone) Zone {
	newFullZone := make(map[FullZoneKey]*RateLimitEntry)
	for _, podZone := range zonePerPod {
		for _, entity := range podZone.RateLimitEntries {
			fmt.Println("aggregateZones: podZone.RateLimitHeader", podZone.RateLimitHeader)
			hashID := FullZoneKey{
				Zone: podZone.Name,
				Key:  entity.Key.String(podZone.RateLimitHeader),
			}
			if entry, exists := newFullZone[hashID]; exists {
				entry.Excess += entity.Excess
				entry.Last = max(entry.Last, entity.Last)
			} else {
				newFullZone[hashID] = &RateLimitEntry{
					Key:    entity.Key,
					Last:   entity.Last,
					Excess: entity.Excess,
				}
			}
		}
	}
	zone := Zone{
		Name:             zonePerPod[0].Name,
		RateLimitHeader:  zonePerPod[0].RateLimitHeader,
		RateLimitEntries: make([]RateLimitEntry, 0, len(newFullZone)),
	}
	for _, entry := range newFullZone {
		zone.RateLimitEntries = append(zone.RateLimitEntries, *entry)
	}
	return zone
}

func max(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}
