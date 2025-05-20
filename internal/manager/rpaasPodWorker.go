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
}

func NewRpaasPodWorker(podURL, podName, rpaasInstanceName, rpaasServiceName string, logger *slog.Logger, zoneDataChan chan Optional[ratelimit.Zone]) *RpaasPodWorker {
	podLogger := logger.With("podName", podName, "podURL", podURL)
	return &RpaasPodWorker{
		PodURL:            podURL,
		PodName:           podName,
		RpaasInstanceName: rpaasInstanceName,
		RpaasServiceName:  rpaasServiceName,
		zoneDataChan:      zoneDataChan,
		logger:            podLogger,
		ReadZoneChan:      make(chan string),
		WriteZoneChan:     make(chan ratelimit.Zone),
		StopChan:          make(chan struct{}),
	}
}

func (w *RpaasPodWorker) Start() {
	go w.Work()
}

func (w *RpaasPodWorker) Stop() {
	if w.StopChan != nil {
		w.StopChan <- struct{}{}
	}
}

func (w *RpaasPodWorker) GetID() string {
	return w.PodName
}

func (w *RpaasPodWorker) Work() {
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
	readLatencyHistogramVec.WithLabelValues(w.PodName, w.RpaasServiceName, w.RpaasInstanceName, zone).Observe(reqDuration.Seconds())
	if reqDuration > config.Spec.WarnZoneReadTime {
		w.logger.Warn("Request took too long", "duration", reqDuration, "zone", zone, "contentLength", response.ContentLength)
	}
	if err != nil {
		return ratelimit.Zone{}, err
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
		return ratelimit.Zone{}, err
	}
	for {
		var message ratelimit.RateLimitEntry
		if err := decoder.Decode(&message); err != nil {
			if err == io.EOF {
				break
			}
			w.logger.Error("Error decoding entry", "error", err)
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
