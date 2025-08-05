package manager

import (
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"time"

	"github.com/vmihailenco/msgpack/v5"

	"github.com/tsuru/rate-limit-control-plane/internal/config"
	"github.com/tsuru/rate-limit-control-plane/internal/ratelimit"
)

type RpaasPodData struct {
	Name string
	URL  string
}

type RpaasPodWorker struct {
	RpaasPodData
	RpaasInstanceData
	logger                   *slog.Logger
	zoneDataChan             chan Optional[ratelimit.Zone]
	ReadZoneChan             chan string
	WriteZoneChan            chan ratelimit.Zone
	StopChan                 chan struct{}
	roundSmallestLastPerZone map[string]int64
	lastHeaderPerZone        map[string]ratelimit.RateLimitHeader
	client                   *http.Client
}

func NewRpaasPodWorker(rpaasPodData RpaasPodData, rpaasInstanceData RpaasInstanceData, logger *slog.Logger, zoneDataChan chan Optional[ratelimit.Zone]) *RpaasPodWorker {
	podLogger := logger.With("podName", rpaasPodData.Name, "podURL", rpaasPodData.URL)
	transport := &http.Transport{
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 10,
		IdleConnTimeout:     90 * time.Second,
		DialContext: (&net.Dialer{
			Timeout:   5 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		TLSHandshakeTimeout: 10 * time.Second,
	}
	client := &http.Client{
		Transport: transport,
		Timeout:   10 * time.Second,
	}
	return &RpaasPodWorker{
		RpaasPodData:             rpaasPodData,
		RpaasInstanceData:        rpaasInstanceData,
		zoneDataChan:             zoneDataChan,
		logger:                   podLogger,
		ReadZoneChan:             make(chan string),
		WriteZoneChan:            make(chan ratelimit.Zone),
		StopChan:                 make(chan struct{}),
		roundSmallestLastPerZone: make(map[string]int64),
		lastHeaderPerZone:        make(map[string]ratelimit.RateLimitHeader),
		client:                   client,
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
	return w.Name
}

func (w *RpaasPodWorker) Work() {
	for {
		select {
		case zoneName := <-w.ReadZoneChan:
			go func() {
				zoneData, err := w.getZoneData(zoneName)
				if err != nil {
					w.zoneDataChan <- Optional[ratelimit.Zone]{Value: zoneData, Error: fmt.Errorf("error getting zone data from pod worker %s: %w", w.Name, err)}
				} else {
					w.logger.Debug("Zone data retrieved", "zone", zoneName, "pod", w.Name, "entries", zoneData.RateLimitEntries)
					w.zoneDataChan <- Optional[ratelimit.Zone]{Value: zoneData, Error: nil}
				}
			}()
		case zone := <-w.WriteZoneChan:
			err := w.sendRequest(zone)
			if err != nil {
				w.logger.Error("Error writing zone data", "zone", zone.Name, "error", err)
			}
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
	endpoint := fmt.Sprintf("%s/rate-limit/%s", w.URL, zone)
	req, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return ratelimit.Zone{}, err
	}
	if w.roundSmallestLastPerZone[zone] != 0 {
		query := req.URL.Query()
		query.Set("last_greater_equal", fmt.Sprintf("%d", w.roundSmallestLastPerZone[zone]-1))
		req.URL.RawQuery = query.Encode()
	}
	start := time.Now()
	response, err := w.client.Do(req)
	if err != nil {
		return ratelimit.Zone{}, fmt.Errorf("error making request to pod %s (%s): %w", w.URL, w.Name, err)
	}
	reqDuration := time.Since(start)
	readLatencyHistogramVec.WithLabelValues(w.Name, w.Service, w.Instance, zone).Observe(reqDuration.Seconds())
	if reqDuration > config.Spec.WarnZoneReadTime {
		w.logger.Warn("Request took too long", "durationMilliseconds", reqDuration.Milliseconds(), "zone", zone, "contentLength", response.ContentLength)
	}
	defer response.Body.Close()
	decoder := msgpack.NewDecoder(response.Body)
	var rateLimitHeader ratelimit.RateLimitHeader
	rateLimitEntries := []ratelimit.RateLimitEntry{}
	if err := decoder.Decode(&rateLimitHeader); err != nil {
		if err == io.EOF {
			w.lastHeaderPerZone[zone] = rateLimitHeader
			return ratelimit.Zone{
				Name:             zone,
				RateLimitHeader:  rateLimitHeader,
				RateLimitEntries: rateLimitEntries,
			}, nil
		}
		w.logger.Error("Error decoding header", "error", err)
		return ratelimit.Zone{}, err
	}
	w.lastHeaderPerZone[zone] = rateLimitHeader
	for {
		var message ratelimit.RateLimitEntry
		if err := decoder.Decode(&message); err != nil {
			if err == io.EOF {
				break
			}
			w.logger.Error("Error decoding entry", "error", err)
			return ratelimit.Zone{}, err
		}
		if w.roundSmallestLastPerZone[zone] == 0 {
			w.roundSmallestLastPerZone[zone] = message.Last
		} else {
			w.roundSmallestLastPerZone[zone] = min(w.roundSmallestLastPerZone[zone], message.Last)
		}
		message.NonMonotic(rateLimitHeader)
		rateLimitEntries = append(rateLimitEntries, message)
	}
	return ratelimit.Zone{
		Name:             zone,
		RateLimitHeader:  rateLimitHeader,
		RateLimitEntries: rateLimitEntries,
	}, nil
}

func min(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}
