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

type RpaasInstanceData struct {
	Instance string
	Service  string
}

type RpaasInstanceSyncWorker struct {
	sync.Mutex
	RpaasInstanceData
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

func NewRpaasInstanceSyncWorker(rpaasInstanceData RpaasInstanceData, zones []string, logger *slog.Logger, notify chan ratelimit.RpaasZoneData, aggregator ZoneAggregator) *RpaasInstanceSyncWorker {
	signals := RpaasInstanceSignals{
		StartRpaasPodWorker: make(chan [2]string),
		StopRpaasPodWorker:  make(chan string),
		StopChan:            make(chan struct{}),
	}
	ticker := time.NewTicker(config.Spec.ControllerIntervalDuration)
	instanceLogger := logger.With("instanceName", rpaasInstanceData.Instance)

	fullZones := make(map[string]map[ratelimit.FullZoneKey]*ratelimit.RateLimitEntry)
	for _, zone := range zones {
		fullZones[zone] = make(map[ratelimit.FullZoneKey]*ratelimit.RateLimitEntry)
	}

	worker := &RpaasInstanceSyncWorker{
		RpaasInstanceData:    rpaasInstanceData,
		RpaasInstanceSignals: signals,
		PodWorkerManager:     NewGoroutineManager(),
		Ticker:               ticker,
		logger:               instanceLogger,
		zoneDataChan:         make(chan Optional[ratelimit.Zone]),
		notify:               notify,
		fullZones:            fullZones,
		aggregator:           aggregator,
	}

	// Initialize instance worker metrics
	activeWorkersGaugeVec.WithLabelValues(rpaasInstanceData.Service, rpaasInstanceData.Instance, "instance").Inc()

	return worker
}

func (w *RpaasInstanceSyncWorker) Work() {
	for {
		select {
		case <-w.Ticker.C:
			w.processTick()
		case <-w.RpaasInstanceSignals.StopChan:
			// Decrement active worker count
			activeWorkersGaugeVec.WithLabelValues(w.Service, w.Instance, "instance").Dec()
			w.cleanup()
			return
		}
	}
}

func (w *RpaasInstanceSyncWorker) processTick() {
	rpaasZoneData := ratelimit.RpaasZoneData{
		RpaasName: w.Instance,
		Data:      []ratelimit.Zone{},
	}
	for zone := range w.fullZones {
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
				aggregationFailuresCounterVec.WithLabelValues(w.Service, w.Instance, zone, "collection_error").Inc()
				continue
			}
			zoneData = append(zoneData, result.Value)
		}
		w.logger.Info("Collected zone data", "zone", zone, "entries", zoneData)
		operationDuration := time.Since(operationStart)
		if operationDuration > config.Spec.WarnZoneCollectionTime {
			w.logger.Warn("Zone data collection took too long", "durationMilliseconds", operationDuration.Milliseconds(), "zone", zone)
		}

		if len(zoneData) == 0 {
			w.Unlock()
			continue
		}

		// Aggregate zone data
		operationStart = time.Now()
		aggregatedZone, newFullZone := w.aggregator.AggregateZones(zoneData, w.fullZones[zone])
		operationDuration = time.Since(operationStart)
		aggregateLatencyHistogramVec.WithLabelValues(w.Service, w.Instance, zone).Observe(operationDuration.Seconds())
		if operationDuration > config.Spec.WarnZoneAggregationTime {
			w.logger.Warn("Zone data aggregation took too long", "duration", operationDuration, "zone", zone, "entries", len(aggregatedZone.RateLimitEntries))
		}
		if config.Spec.FeatureFlagPersistAggregatedData {
			w.fullZones[zone] = newFullZone
		}
		w.Unlock()

		if operationDuration > config.Spec.WarnZoneAggregationTime {
			w.logger.Warn("Zone data aggregation took too long", "durationMilliseconds", operationDuration.Milliseconds(), "zone", zone, "entries", len(aggregatedZone.RateLimitEntries))
		}
		w.logger.Debug("Aggregated zone data", "zone", zone, "entries", aggregatedZone.RateLimitEntries)

		rpaasZoneData.Data = append(rpaasZoneData.Data, aggregatedZone)

		// Record rate limit entries metrics
		rateLimitEntriesCounterVec.WithLabelValues(w.Service, w.Instance, zone, "aggregated").Add(float64(len(aggregatedZone.RateLimitEntries)))

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
	return w.Instance
}

func (w *RpaasInstanceSyncWorker) AddPodWorker(podIP, podName string) {
	rpaasPodData := RpaasPodData{
		Name: podName,
		URL:  fmt.Sprintf("http://%s:%d", podIP, administrativePort),
	}
	podWorker := NewRpaasPodWorker(rpaasPodData, w.RpaasInstanceData, w.logger, w.zoneDataChan)
	w.PodWorkerManager.AddWorker(podWorker)
}

func (w *RpaasInstanceSyncWorker) RemovePodWorker(podName string) error {
	if ok := w.PodWorkerManager.RemoveWorker(podName); !ok {
		return fmt.Errorf("pod worker not found: %s", podName)
	}
	return nil
}

func (w *RpaasInstanceSyncWorker) CountWorkers() int {
	w.Lock()
	defer w.Unlock()
	return len(w.PodWorkerManager.ListWorkerIDs())
}
