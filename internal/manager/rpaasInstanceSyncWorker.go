package manager

import (
	"encoding/base64"
	"fmt"
	"log"
	"sync"
	"time"
)

type RpaasInstanceSyncWorker struct {
	sync.Mutex
	RpaasInstanceName    string
	Zones                []string
	RpaasInstanceSignals RpaasInstanceSignals
	RpaasPodWorkers      map[string]*RpaasPodWorkerSignals
	Ticker               *time.Ticker
	zoneDataChan         chan Optional[Zone]
	fullZone             map[FullZoneKey]*RateLimitEntry
}

type RpaasInstanceSignals struct {
	StartRpaasPodWorker chan [2]string
	StopRpaasPodWorker  chan string
	StopChan            chan struct{}
}

func NewRpaasInstanceSyncWorker(rpaasInstanceName string, zones []string) *RpaasInstanceSyncWorker {
	signals := RpaasInstanceSignals{
		StartRpaasPodWorker: make(chan [2]string),
		StopRpaasPodWorker:  make(chan string),
	}
	// TODO: Correctly set the ticker duration using config
	ticker := time.NewTicker(10 * time.Second)
	return &RpaasInstanceSyncWorker{
		RpaasInstanceName:    rpaasInstanceName,
		Zones:                zones,
		RpaasInstanceSignals: signals,
		RpaasPodWorkers:      make(map[string]*RpaasPodWorkerSignals),
		Ticker:               ticker,
		zoneDataChan:         make(chan Optional[Zone]),
		fullZone:             make(map[FullZoneKey]*RateLimitEntry),
	}
}

func (w *RpaasInstanceSyncWorker) Work() {
	for {
		select {
		case rpaasPodWorkerKeys := <-w.RpaasInstanceSignals.StartRpaasPodWorker:
			w.Lock()
			if _, exists := w.RpaasPodWorkers[rpaasPodWorkerKeys[0]]; exists {
				w.Unlock()
				continue
			}
			podWorker := NewRpaasPodWorker(rpaasPodWorkerKeys[1], w.zoneDataChan)
			w.RpaasPodWorkers[rpaasPodWorkerKeys[0]] = &podWorker.RpaasPodWorkerSignals
			w.Unlock()
			go podWorker.Work()
		case rpaasPodWorkerKey := <-w.RpaasInstanceSignals.StopRpaasPodWorker:
			w.Lock()
			podWorkerSignals, exists := w.RpaasPodWorkers[rpaasPodWorkerKey]
			if !exists {
				w.Unlock()
				continue
			}
			podWorkerSignals.StopChan <- struct{}{}
		case <-w.Ticker.C:
			for _, zone := range w.Zones {
				w.Lock()
				workerNum := len(w.RpaasPodWorkers)
				for _, podWorker := range w.RpaasPodWorkers {
					podWorker.ReadZoneChan <- zone
				}
				zoneData := []Zone{}
				for i := 0; i < workerNum; i++ {
					result := <-w.zoneDataChan
					if result.Error != nil {
						log.Fatal(result.Error)
					}
					zoneData = append(zoneData, result.Value)
				}
				log.Printf("zoneData %+v\n", zoneData)
				aggregatedZones := w.aggregateZones(zoneData)
				log.Println("aggregateZones", aggregatedZones)
				w.Unlock()
				for _, podWorker := range w.RpaasPodWorkers {
					podWorker.WriteZoneChan <- aggregatedZones
				}
			}
		case <-w.RpaasInstanceSignals.StopChan:
			w.Lock()
			for _, podWorker := range w.RpaasPodWorkers {
				podWorker.StopChan <- struct{}{}
			}
			w.Unlock()
			close(w.RpaasInstanceSignals.StartRpaasPodWorker)
			close(w.RpaasInstanceSignals.StopRpaasPodWorker)
			close(w.RpaasInstanceSignals.StopChan)
			close(w.zoneDataChan)
			w.Ticker.Stop()
			return

		}
	}
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
