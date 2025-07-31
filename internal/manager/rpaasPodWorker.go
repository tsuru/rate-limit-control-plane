package manager

import (
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/vmihailenco/msgpack/v5"

	"github.com/tsuru/rate-limit-control-plane/internal/config"
	"github.com/tsuru/rate-limit-control-plane/internal/ratelimit"
)

type RpaasPodWorker struct {
	PodURL            string
	RpaasInstanceName string
	RpaasServiceName  string
	PodName           string
	logger            *slog.Logger
	zoneDataChan      chan Optional[ratelimit.Zone]
	ReadZoneChan      chan string
	WriteZoneChan     chan ratelimit.Zone
	StopChan          chan struct{}
	RoundSmallestLast int64
	startTime         time.Time
}

func NewRpaasPodWorker(podURL, podName, rpaasInstanceName, rpaasServiceName string, logger *slog.Logger, zoneDataChan chan Optional[ratelimit.Zone]) *RpaasPodWorker {
	podLogger := logger.With("podName", podName, "podURL", podURL)
	worker := &RpaasPodWorker{
		PodURL:            podURL,
		PodName:           podName,
		RpaasInstanceName: rpaasInstanceName,
		RpaasServiceName:  rpaasServiceName,
		zoneDataChan:      zoneDataChan,
		logger:            podLogger,
		ReadZoneChan:      make(chan string),
		WriteZoneChan:     make(chan ratelimit.Zone),
		StopChan:          make(chan struct{}),
		startTime:         time.Now(),
	}

	// Initialize worker metrics
	activeWorkersGaugeVec.WithLabelValues(rpaasServiceName, "pod").Inc()
	workerUptimeGaugeVec.WithLabelValues(podName, "pod", rpaasInstanceName, rpaasServiceName).Set(0)

	return worker
}

func (w *RpaasPodWorker) Start() {
	go w.Work()
}

func (w *RpaasPodWorker) Stop() {
	if w.StopChan != nil {
		w.StopChan <- struct{}{}
		// Decrement active worker count
		activeWorkersGaugeVec.WithLabelValues(w.RpaasServiceName, "pod").Dec()
	}
}

func (w *RpaasPodWorker) GetID() string {
	return w.PodName
}

func (w *RpaasPodWorker) Work() {
	// Update worker uptime periodically
	uptimeTicker := time.NewTicker(30 * time.Second)
	defer uptimeTicker.Stop()

	for {
		select {
		case zoneName := <-w.ReadZoneChan:
			go func() {
				zoneData, err := w.getZoneData(zoneName)
				if err != nil {
					w.zoneDataChan <- Optional[ratelimit.Zone]{Value: zoneData, Error: fmt.Errorf("error getting zone data from pod worker %s: %w", w.PodName, err)}
				} else {
					w.zoneDataChan <- Optional[ratelimit.Zone]{Value: zoneData, Error: nil}
				}
			}()
		case <-w.WriteZoneChan:
			// TODO: Implement the logic to write zone data to the pod
		case <-uptimeTicker.C:
			uptime := time.Since(w.startTime).Seconds()
			workerUptimeGaugeVec.WithLabelValues(w.PodName, "pod", w.RpaasInstanceName, w.RpaasServiceName).Set(uptime)
		case <-w.StopChan:
			w.cleanup()
			return
		}
	}
}

func (w *RpaasPodWorker) cleanup() {
	close(w.ReadZoneChan)
	close(w.WriteZoneChan)
	close(w.StopChan)
}

func (w *RpaasPodWorker) getZoneData(zone string) (ratelimit.Zone, error) {
	endpoint := fmt.Sprintf("%s/%s/%s", w.PodURL, "rate-limit", zone)
	req, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return ratelimit.Zone{}, err
	}
	if w.RoundSmallestLast != 0 {
		query := req.URL.Query()
		query.Set("last_greater_equal", fmt.Sprintf("%d", w.RoundSmallestLast))
		req.URL.RawQuery = query.Encode()
	}
	start := time.Now()
	response, err := http.DefaultClient.Do(req)
	reqDuration := time.Since(start)

	if err != nil {
		// Record failed operation
		readOperationsCounterVec.WithLabelValues(w.PodName, w.RpaasServiceName, w.RpaasInstanceName, zone, "error").Inc()
		podHealthStatusGaugeVec.WithLabelValues(w.PodName, w.RpaasServiceName, w.RpaasInstanceName, zone).Set(0)
		return ratelimit.Zone{}, fmt.Errorf("error making request to pod %s (%s): %w", w.PodURL, w.PodName, err)
	}

	// Record successful operation and latency
	readOperationsCounterVec.WithLabelValues(w.PodName, w.RpaasServiceName, w.RpaasInstanceName, zone, "success").Inc()
	readLatencyHistogramVec.WithLabelValues(w.PodName, w.RpaasServiceName, w.RpaasInstanceName, zone).Observe(reqDuration.Seconds())
	podHealthStatusGaugeVec.WithLabelValues(w.PodName, w.RpaasServiceName, w.RpaasInstanceName, zone).Set(1)
	if reqDuration > config.Spec.WarnZoneReadTime {
		w.logger.Warn("Request took too long", "duration", reqDuration, "zone", zone, "contentLength", response.ContentLength)
	}
	defer response.Body.Close()
	decoder := msgpack.NewDecoder(response.Body)
	var rateLimitHeader ratelimit.RateLimitHeader
	rateLimitEntries := []ratelimit.RateLimitEntry{}
	if err := decoder.Decode(&rateLimitHeader); err != nil {
		if err == io.EOF {
			return ratelimit.Zone{
				Name:             zone,
				RateLimitHeader:  rateLimitHeader,
				RateLimitEntries: rateLimitEntries,
			}, nil
		}
		w.logger.Error("Error decoding header", "error", err)
		readOperationsCounterVec.WithLabelValues(w.PodName, w.RpaasServiceName, w.RpaasInstanceName, zone, "error").Inc()
		return ratelimit.Zone{}, err
	}
	for {
		var message ratelimit.RateLimitEntry
		if err := decoder.Decode(&message); err != nil {
			if err == io.EOF {
				break
			}
			w.logger.Error("Error decoding entry", "error", err)
			readOperationsCounterVec.WithLabelValues(w.PodName, w.RpaasServiceName, w.RpaasInstanceName, zone, "error").Inc()
			return ratelimit.Zone{}, err
		}
		if w.RoundSmallestLast == 0 {
			w.RoundSmallestLast = message.Last
		} else {
			w.RoundSmallestLast = min(w.RoundSmallestLast, message.Last)
		}
		message.Last = toNonMonotonic(message.Last, rateLimitHeader)
		rateLimitEntries = append(rateLimitEntries, message)
	}
	w.logger.Debug("Received rate limit entries", "zone", zone, "entries", len(rateLimitEntries))

	// Record zone data size and rate limit rules count
	zoneDataSize := response.ContentLength
	if zoneDataSize > 0 {
		zoneDataSizeHistogramVec.WithLabelValues(w.RpaasInstanceName, w.RpaasServiceName, zone).Observe(float64(zoneDataSize))
	}
	rateLimitRulesActiveGaugeVec.WithLabelValues(w.RpaasInstanceName, w.RpaasServiceName, zone).Set(float64(len(rateLimitEntries)))

	return ratelimit.Zone{
		Name:             zone,
		RateLimitHeader:  rateLimitHeader,
		RateLimitEntries: rateLimitEntries,
	}, nil
}

func toNonMonotonic(last int64, header ratelimit.RateLimitHeader) int64 {
	return header.Now - (header.NowMonotonic - last)
}

func min(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}
