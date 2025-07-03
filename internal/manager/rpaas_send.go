package manager

import (
	"bytes"
	"fmt"
	"net/http"

	"github.com/tsuru/rate-limit-control-plane/internal/ratelimit"
	"github.com/vmihailenco/msgpack/v5"
)

func (w *RpaasPodWorker) sendRequest(header ratelimit.RateLimitHeader, entries []ratelimit.RateLimitEntry, endpoint string) error {
	var buf bytes.Buffer
	encoder := msgpack.NewEncoder(&buf)
	var values []interface{} = []interface{}{
		headerToArray(header),
	}
	for _, entry := range entries {
		values = append(values, entryToArray(entry, header))
	}
	if err := encoder.Encode(values); err != nil {
		return fmt.Errorf("error encoding entries: %w", err)
	}
	req, err := http.NewRequest(http.MethodPost, endpoint, &buf)
	if err != nil {
		return fmt.Errorf("error creating request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-msgpack")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("error sending request to %s: %w", endpoint, err)
	}
	fmt.Println(resp.Status)
	defer resp.Body.Close()
	return nil
}

func headerToArray(header ratelimit.RateLimitHeader) []interface{} {
	return []interface{}{
		header.Key,
		header.Now,
		header.NowMonotonic,
	}
}

func entryToArray(entry ratelimit.RateLimitEntry, header ratelimit.RateLimitHeader) []interface{} {
	return []interface{}{
		entry.Key,
		entry.Last,
		entry.Excess,
	}
}
