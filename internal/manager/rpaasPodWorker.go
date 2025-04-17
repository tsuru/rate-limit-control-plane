package manager

import (
	"fmt"
	"io"
	"log"
	"net/http"

	"github.com/tsuru/rate-limit-control-plane/internal/ratelimit"
	"github.com/vmihailenco/msgpack/v5"
)

const administrativePort = 8800

const (
	binaryRemoteAddress = "$binary_remote_addr"
	remoteAddress       = "$remote_addr"
)

type RpaasPodWorker struct {
	PodIP         string
	PodName       string
	zoneDataChan  chan Optional[ratelimit.Zone]
	ReadZoneChan  chan string
	WriteZoneChan chan ratelimit.Zone
	StopChan      chan struct{}
}

func NewRpaasPodWorker(podIP, podName string, zoneDataChan chan Optional[ratelimit.Zone]) *RpaasPodWorker {
	return &RpaasPodWorker{
		PodIP:         podIP,
		PodName:       podName,
		zoneDataChan:  zoneDataChan,
		ReadZoneChan:  make(chan string),
		WriteZoneChan: make(chan ratelimit.Zone),
		StopChan:      make(chan struct{}),
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
				w.zoneDataChan <- Optional[ratelimit.Zone]{Value: zoneData, Error: err}
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
	endpoint := fmt.Sprintf("http://%s:%d/%s/%s", w.PodIP, administrativePort, "rate-limit", zone)
	req, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return ratelimit.Zone{}, err
	}
	response, err := http.DefaultClient.Do(req)
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
		log.Printf("Pod %s returned an error deconding header: %v", w.PodIP, err)
		return ratelimit.Zone{}, err
	}
	for {
		var message ratelimit.RateLimitEntry
		if err := decoder.Decode(&message); err != nil {
			if err == io.EOF {
				break
			}
			log.Printf("Pod %s returned an error deconding entry: %v", w.PodIP, err)
			return ratelimit.Zone{}, err
		}
		message.Last = toNonMonotonic(message.Last, rateLimitHeader)
		rateLimitEntries = append(rateLimitEntries, message)
	}
	return ratelimit.Zone{
		Name:             zone,
		RateLimitHeader:  rateLimitHeader,
		RateLimitEntries: rateLimitEntries,
	}, nil
}

func toNonMonotonic(last int64, header ratelimit.RateLimitHeader) int64 {
	return header.Now - (header.NowMonotonic - last)
}
