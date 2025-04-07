package manager

import (
	"fmt"
	"io"
	"log"
	"net/http"

	"github.com/vmihailenco/msgpack/v5"
)

const administrativePort = 8800

type RpaasPodWorker struct {
	PodIP                 string
	zoneDataChan          chan Optional[Zone]
	RpaasPodWorkerSignals RpaasPodWorkerSignals
}

type RpaasPodWorkerSignals struct {
	ReadZoneChan  chan string
	WriteZoneChan chan Zone
	StopChan      chan struct{}
}

func NewRpaasPodWorker(podIP string, zoneDataChan chan Optional[Zone]) *RpaasPodWorker {
	podWorkerSignals := RpaasPodWorkerSignals{
		ReadZoneChan:  make(chan string),
		WriteZoneChan: make(chan Zone),
		StopChan:      make(chan struct{}),
	}
	return &RpaasPodWorker{
		PodIP:                 podIP,
		zoneDataChan:          zoneDataChan,
		RpaasPodWorkerSignals: podWorkerSignals,
	}
}

func (w *RpaasPodWorker) Work() {
	for {
		select {
		case zoneName := <-w.RpaasPodWorkerSignals.ReadZoneChan:
			go func() {
				zoneData, err := w.getZoneData(zoneName)
				w.zoneDataChan <- Optional[Zone]{Value: zoneData, Error: err}
			}()
		case zone := <-w.RpaasPodWorkerSignals.WriteZoneChan:
			fmt.Println("Writing zone data to pod", zone)
			// TODO: Implement the logic to write zone data to the pod
		case <-w.RpaasPodWorkerSignals.StopChan:
			close(w.RpaasPodWorkerSignals.ReadZoneChan)
			close(w.RpaasPodWorkerSignals.WriteZoneChan)
			close(w.RpaasPodWorkerSignals.StopChan)
			close(w.zoneDataChan)
			return
		}
	}
}

func (w *RpaasPodWorker) getZoneData(zone string) (Zone, error) {
	endpoint := fmt.Sprintf("http://%s:%d/%s/%s", w.PodIP, administrativePort, "rate-limit", zone)
	req, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return Zone{}, err
	}
	response, err := http.DefaultClient.Do(req)
	if err != nil {
		return Zone{}, err
	}
	defer response.Body.Close()
	decoder := msgpack.NewDecoder(response.Body)
	var rateLimitHeader RateLimitHeader
	rateLimitEntries := []RateLimitEntry{}
	if err := decoder.Decode(&rateLimitHeader); err != nil {
		if err == io.EOF {
			return Zone{
				Name:             zone,
				RateLimitHeader:  rateLimitHeader,
				RateLimitEntries: rateLimitEntries,
			}, nil
		}
		log.Printf("Pod %s returned an error deconding header: %v", w.PodIP, err)
		return Zone{}, err
	}
	for {
		var message RateLimitEntry
		if err := decoder.Decode(&message); err != nil {
			if err == io.EOF {
				break
			}
			log.Printf("Pod %s returned an error deconding entry: %v", w.PodIP, err)
			return Zone{}, err
		}
		rateLimitEntries = append(rateLimitEntries, message)
	}
	return Zone{
		Name:             zone,
		RateLimitHeader:  rateLimitHeader,
		RateLimitEntries: rateLimitEntries,
	}, nil
}
