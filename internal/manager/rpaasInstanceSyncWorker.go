package manager

import (
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/tsuru/rate-limit-control-plane/internal/config"
	"github.com/tsuru/rate-limit-control-plane/internal/ratelimit"
)

const administrativePort = 8800

type ZoneAggregator interface {
	AggregateZones(zonePerPod []ratelimit.Zone, fullZone map[ratelimit.FullZoneKey]*ratelimit.RateLimitEntry) (ratelimit.Zone, map[ratelimit.FullZoneKey]*ratelimit.RateLimitEntry)
}

// Update RpaasInstanceSyncWorker to use GoroutineManager for managing pod workers
type RpaasInstanceSyncWorker struct {
	sync.Mutex
	RpaasInstanceName    string
	RpaasServiceName     string
	Zones                []string
	RpaasInstanceSignals RpaasInstanceSignals
	PodWorkerManager     *GoroutineManager
	Ticker               *time.Ticker
	logger               *slog.Logger
	zoneDataChan         chan Optional[ratelimit.Zone]
	notify               chan ratelimit.RpaasZoneData
	fullZones            map[string]map[ratelimit.FullZoneKey]*ratelimit.RateLimitEntry
	aggregator           ZoneAggregator
}

type RpaasInstanceSignals struct {
	StartRpaasPodWorker chan [2]string
	StopRpaasPodWorker  chan string
	StopChan            chan struct{}
}

func NewRpaasInstanceSyncWorker(rpaasInstanceName, rpaasServiceName string, zones []string, logger *slog.Logger, notify chan ratelimit.RpaasZoneData, aggregator ZoneAggregator) *RpaasInstanceSyncWorker {
	signals := RpaasInstanceSignals{
		StartRpaasPodWorker: make(chan [2]string),
		StopRpaasPodWorker:  make(chan string),
		StopChan:            make(chan struct{}),
	}
	ticker := time.NewTicker(config.Spec.ControllerMinutesInternal)
	instanceLogger := logger.With("instanceName", rpaasInstanceName)

	return &RpaasInstanceSyncWorker{
		RpaasInstanceName:    rpaasInstanceName,
		RpaasServiceName:     rpaasServiceName,
		Zones:                zones,
		RpaasInstanceSignals: signals,
		PodWorkerManager:     NewGoroutineManager(),
		Ticker:               ticker,
		logger:               instanceLogger,
		zoneDataChan:         make(chan Optional[ratelimit.Zone]),
		notify:               notify,
		fullZones:            make(map[string]map[ratelimit.FullZoneKey]*ratelimit.RateLimitEntry),
		aggregator:           aggregator,
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
	rpaasZoneData := ratelimit.RpaasZoneData{
		RpaasName: w.RpaasInstanceName,
		Data:      []ratelimit.Zone{},
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
		zoneData := []ratelimit.Zone{}
		operationStart := time.Now()
		for range workerCount {
			result := <-w.zoneDataChan
			if result.Error != nil {
				w.logger.Error("Error getting zone data", "error", result.Error)
				continue
			}
			zoneData = append(zoneData, result.Value)
		}
		operationDuration := time.Since(operationStart)
		if operationDuration > config.Spec.WarnZoneCollectionTime {
			w.logger.Warn("Zone data collection took too long", "duration", operationDuration.Milliseconds(), "zone", zone)
		}

		if len(zoneData) == 0 {
			w.Unlock()
			continue
		}

		// Aggregate zone data
		operationStart = time.Now()
		aggregatedZone, newFullZone := w.aggregator.AggregateZones(zoneData, w.fullZones[zone])
		operationDuration = time.Since(operationStart)
		aggregateLatencyHistogramVec.WithLabelValues(w.RpaasInstanceName, w.RpaasServiceName, zone).Observe(operationDuration.Seconds())
		if operationDuration > config.Spec.WarnZoneAggregationTime {
			w.logger.Warn("Zone data aggregation took too long", "duration", operationDuration, "zone", zone, "entries", len(aggregatedZone.RateLimitEntries))
		}
		if config.Spec.FeatureFlagPersistAggregatedData {
			w.fullZones[zone] = newFullZone
		}
		w.Unlock()

		rpaasZoneData.Data = append(rpaasZoneData.Data, aggregatedZone)

		if config.Spec.FeatureFlagPersistAggregatedData {
			// Write aggregated data back to pod workers
			w.PodWorkerManager.ForEachWorker(func(worker Worker) {
				if podWorker, ok := worker.(*RpaasPodWorker); ok {
					podWorker.WriteZoneChan <- aggregatedZone
				}
			})
		}
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
	podURL := fmt.Sprintf("http://%s:%d", podIP, administrativePort)
	podWorker := NewRpaasPodWorker(podURL, podName, w.RpaasInstanceName, w.RpaasServiceName, w.logger, w.zoneDataChan)
	w.PodWorkerManager.AddWorker(podWorker)
}

func (w *RpaasInstanceSyncWorker) RemovePodWorker(podName string) error {
	if ok := w.PodWorkerManager.RemoveWorker(podName); !ok {
		return fmt.Errorf("pod worker not found: %s", podName)
	}
	return nil
}
