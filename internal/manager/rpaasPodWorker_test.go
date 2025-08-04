package manager

import (
	"fmt"
	"log/slog"
	"net"
	"sort"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/tsuru/rate-limit-control-plane/internal/aggregator"
	"github.com/tsuru/rate-limit-control-plane/internal/ratelimit"
	"github.com/tsuru/rate-limit-control-plane/test"
)

const (
	instanceName = "test-instance"
	serviceName  = "rpaas"
)

type loggerSpy struct {
	LastMessage   string
	NumberOfCalls int
}

func (l *loggerSpy) Write(p []byte) (int, error) {
	l.NumberOfCalls++
	l.LastMessage = string(p)
	return len(p), nil
}

func (l *loggerSpy) Clean() {
	l.NumberOfCalls = 0
	l.LastMessage = ""
}

func TestRpaasPodWorkerAggregationWithoutPreviousData(t *testing.T) {
	logSpy := new(loggerSpy)
	logHandler := slog.NewTextHandler(logSpy, nil)
	zone := "one"
	completeAggregator := &aggregator.CompleteAggregator{}
	rpaasInstanceData := RpaasInstanceData{
		Instance: instanceName,
		Service:  serviceName,
	}

	listener1, err := net.Listen("tcp", ":0")
	require.NoError(t, err)
	defer listener1.Close()

	repository1 := test.NewRepository()
	go test.NewServerMock(listener1, repository1)

	listener2, err := net.Listen("tcp", ":0")
	require.NoError(t, err)
	defer listener2.Close()

	repository2 := test.NewRepository()
	go test.NewServerMock(listener2, repository2)

	zoneDataChan := make(chan Optional[ratelimit.Zone])
	// listener1.Addr().String() is in format "[::]:569393" change to "http://localhost:569393"
	_, port1, err := net.SplitHostPort(listener1.Addr().String())
	require.NoError(t, err)
	rpaasPodData1 := RpaasPodData{
		Name: instanceName,
		URL:  fmt.Sprintf("http://localhost:%s", port1),
	}
	podWorker1 := NewRpaasPodWorker(rpaasPodData1, rpaasInstanceData, slog.New(logHandler), zoneDataChan)

	_, port2, err := net.SplitHostPort(listener2.Addr().String())
	require.NoError(t, err)
	rpaasPodData2 := RpaasPodData{
		Name: instanceName,
		URL:  fmt.Sprintf("http://localhost:%s", port2),
	}
	podWorker2 := NewRpaasPodWorker(rpaasPodData2, rpaasInstanceData, slog.New(logHandler), zoneDataChan)

	go podWorker1.Start()
	defer podWorker1.Stop()
	go podWorker2.Start()
	defer podWorker2.Stop()

	now := time.Now().UTC().UnixMilli()
	repo1Data := []*test.Body{
		{
			Key:    []byte("192.168.0.1"),
			Last:   now - 50,
			Excess: 500,
		},
		{
			Key:    []byte("192.168.0.2"),
			Last:   now - 100,
			Excess: 520,
		},
		{
			Key:    []byte("192.168.0.3"),
			Last:   now - 150,
			Excess: 480,
		},
	}
	repo2Data := []*test.Body{
		{
			Key:    []byte("192.168.0.1"),
			Last:   now - 25,
			Excess: 530,
		},
		{
			Key:    []byte("192.168.0.3"),
			Last:   now - 300,
			Excess: 510,
		},
	}

	expectedAggregatedZone := ratelimit.Zone{
		Name: "one",
		RateLimitHeader: ratelimit.RateLimitHeader{
			Key: ratelimit.RemoteAddress,
		},
		RateLimitEntries: []ratelimit.RateLimitEntry{
			{
				Key:    []byte("192.168.0.1"),
				Last:   now - 25,
				Excess: 2060,
			},
			{
				Key:    []byte("192.168.0.2"),
				Last:   now - 100,
				Excess: 1040,
			},
			{
				Key:    []byte("192.168.0.3"),
				Last:   now - 150,
				Excess: 1980,
			},
		},
	}

	expectedFullZone := map[ratelimit.FullZoneKey]*ratelimit.RateLimitEntry{
		{Zone: "one", Key: "192.168.0.1"}: {Key: []byte("192.168.0.1"), Excess: 2060, Last: now - 25},
		{Zone: "one", Key: "192.168.0.2"}: {Key: []byte("192.168.0.2"), Excess: 1040, Last: now - 100},
		{Zone: "one", Key: "192.168.0.3"}: {Key: []byte("192.168.0.3"), Excess: 1980, Last: now - 150},
	}

	setRepositoryData(repository1, zone, repo1Data)
	setRepositoryData(repository2, zone, repo2Data)

	workersZoneData := []ratelimit.Zone{}

	podWorker1.ReadZoneChan <- zone
	zoneData1 := <-zoneDataChan
	require.NoError(t, zoneData1.Error)
	checkZoneDataAgainstRepoData(t, zoneData1.Value, repo1Data)
	workersZoneData = append(workersZoneData, zoneData1.Value)

	podWorker2.ReadZoneChan <- zone
	zoneData2 := <-zoneDataChan
	require.NoError(t, zoneData2.Error)
	checkZoneDataAgainstRepoData(t, zoneData2.Value, repo2Data)
	workersZoneData = append(workersZoneData, zoneData2.Value)

	aggregatedZone, fullZone := completeAggregator.AggregateZones(workersZoneData, nil)
	require.Equal(t, expectedAggregatedZone.Name, aggregatedZone.Name)
	require.Equal(t, expectedAggregatedZone.RateLimitHeader.Key, aggregatedZone.RateLimitHeader.Key)
	require.ElementsMatch(t, expectedAggregatedZone.RateLimitEntries, aggregatedZone.RateLimitEntries)
	require.Equal(t, expectedFullZone, fullZone)
}

func TestRpaasPodWorkerAggregationWithPreviousData(t *testing.T) {
	logSpy := new(loggerSpy)
	logHandler := slog.NewTextHandler(logSpy, nil)
	zone := "one"
	completeAggregator := &aggregator.CompleteAggregator{}
	rpaasInstanceData := RpaasInstanceData{
		Instance: instanceName,
		Service:  serviceName,
	}

	listener1, err := net.Listen("tcp", ":0")
	require.NoError(t, err)
	defer listener1.Close()

	repository1 := test.NewRepository()
	go test.NewServerMock(listener1, repository1)

	listener2, err := net.Listen("tcp", ":0")
	require.NoError(t, err)
	defer listener2.Close()

	repository2 := test.NewRepository()
	go test.NewServerMock(listener2, repository2)

	zoneDataChan := make(chan Optional[ratelimit.Zone])
	_, port1, err := net.SplitHostPort(listener1.Addr().String())
	require.NoError(t, err)
	rpaasPodData1 := RpaasPodData{
		Name: instanceName,
		URL:  fmt.Sprintf("http://localhost:%s", port1),
	}
	podWorker1 := NewRpaasPodWorker(rpaasPodData1, rpaasInstanceData, slog.New(logHandler), zoneDataChan)

	_, port2, err := net.SplitHostPort(listener2.Addr().String())
	require.NoError(t, err)
	rpaasPodData2 := RpaasPodData{
		Name: instanceName,
		URL:  fmt.Sprintf("http://localhost:%s", port2),
	}
	podWorker2 := NewRpaasPodWorker(rpaasPodData2, rpaasInstanceData, slog.New(logHandler), zoneDataChan)

	go podWorker1.Start()
	defer podWorker1.Stop()
	go podWorker2.Start()
	defer podWorker2.Stop()

	now := time.Now().UTC().UnixMilli()
	repo1Data := []*test.Body{
		{
			Key:    []byte("192.168.0.1"),
			Last:   now - 80,
			Excess: 1100,
		},
		{
			Key:    []byte("192.168.0.2"),
			Last:   now - 100,
			Excess: 520,
		},
		{
			Key:    []byte("192.168.0.3"),
			Last:   now - 180,
			Excess: 1000,
		},
	}
	repo2Data := []*test.Body{
		{
			Key:    []byte("192.168.0.1"),
			Last:   now - 55,
			Excess: 1050,
		},
		{
			Key:    []byte("192.168.0.2"),
			Last:   now - 100,
			Excess: 520,
		},
		{
			Key:    []byte("192.168.0.3"),
			Last:   now - 330,
			Excess: 900,
		},
	}

	previousFullZone := map[ratelimit.FullZoneKey]*ratelimit.RateLimitEntry{
		{Zone: "one", Key: "192.168.0.1"}: {Key: []byte("192.168.0.1"), Excess: 1030, Last: now - 25},
		{Zone: "one", Key: "192.168.0.2"}: {Key: []byte("192.168.0.2"), Excess: 520, Last: now - 100},
		{Zone: "one", Key: "192.168.0.3"}: {Key: []byte("192.168.0.3"), Excess: 990, Last: now - 150},
	}

	expectedAggregatedZone := ratelimit.Zone{
		Name: "one",
		RateLimitHeader: ratelimit.RateLimitHeader{
			Key: ratelimit.RemoteAddress,
		},
		RateLimitEntries: []ratelimit.RateLimitEntry{
			{
				Key:    []byte("192.168.0.1"),
				Last:   now - 55,
				Excess: 1210,
			},
			{
				Key:    []byte("192.168.0.2"),
				Last:   now - 100,
				Excess: 520,
			},
			{
				Key:    []byte("192.168.0.3"),
				Last:   now - 180,
				Excess: 965,
			},
		},
	}

	expectedFullZone := map[ratelimit.FullZoneKey]*ratelimit.RateLimitEntry{
		{Zone: "one", Key: "192.168.0.1"}: {Key: []byte("192.168.0.1"), Excess: 1210, Last: now - 55},
		{Zone: "one", Key: "192.168.0.2"}: {Key: []byte("192.168.0.2"), Excess: 520, Last: now - 100},
		{Zone: "one", Key: "192.168.0.3"}: {Key: []byte("192.168.0.3"), Excess: 965, Last: now - 180},
	}

	setRepositoryData(repository1, zone, repo1Data)
	setRepositoryData(repository2, zone, repo2Data)

	workersZoneData := []ratelimit.Zone{}

	podWorker1.ReadZoneChan <- zone
	zoneData1 := <-zoneDataChan
	require.NoError(t, zoneData1.Error)
	checkZoneDataAgainstRepoData(t, zoneData1.Value, repo1Data)
	workersZoneData = append(workersZoneData, zoneData1.Value)

	podWorker2.ReadZoneChan <- zone
	zoneData2 := <-zoneDataChan
	require.NoError(t, zoneData2.Error)
	checkZoneDataAgainstRepoData(t, zoneData2.Value, repo2Data)
	workersZoneData = append(workersZoneData, zoneData2.Value)

	aggregatedZone, fullZone := completeAggregator.AggregateZones(workersZoneData, previousFullZone)
	require.Equal(t, expectedAggregatedZone.Name, aggregatedZone.Name)
	require.Equal(t, expectedAggregatedZone.RateLimitHeader.Key, aggregatedZone.RateLimitHeader.Key)
	require.ElementsMatch(t, expectedAggregatedZone.RateLimitEntries, aggregatedZone.RateLimitEntries)
	require.Equal(t, expectedFullZone, fullZone)
}

func setRepositoryData(repository *test.Repositories, zone string, data []*test.Body) {
	repository.SetRateLimit(zone, data)
}

func checkZoneDataAgainstRepoData(t *testing.T, zoneData ratelimit.Zone, repoData []*test.Body) {
	t.Helper()
	require.Len(t, zoneData.RateLimitEntries, len(repoData))

	sort.Slice(zoneData.RateLimitEntries, func(i, j int) bool {
		return string(zoneData.RateLimitEntries[i].Key) <= string(zoneData.RateLimitEntries[j].Key)
	})
	sort.Slice(repoData, func(i, j int) bool {
		return string(repoData[i].Key) <= string(repoData[j].Key)
	})

	for i := range repoData {
		zoneDataElement := zoneData.RateLimitEntries[i]
		repoDataElement := repoData[i]
		require.EqualValues(t, repoDataElement.Key, zoneDataElement.Key)
		require.EqualValues(t, repoDataElement.Last, zoneDataElement.Last)
		require.EqualValues(t, repoDataElement.Excess, zoneDataElement.Excess)
	}
}
