package manager

import (
	"bytes"
	"fmt"
	"net/http"

	"github.com/tsuru/rate-limit-control-plane/internal/ratelimit"
	"github.com/vmihailenco/msgpack/v5"
)

func (w *RpaasPodWorker) sendRequest(zone ratelimit.Zone) error {
	var buf bytes.Buffer
	endpoint := fmt.Sprintf("%s/%s/%s", w.PodURL, "rate-limit", zone.Name)
	encoder := msgpack.NewEncoder(&buf)

	w.logger.Info("Writing zone data", "zone", zone.Name, "url", endpoint)
	if len(zone.RateLimitEntries) > 0 {
		w.logger.Info("Zone has entries", "zone", zone.Name, "entry[0]", zone.RateLimitEntries[0])
	}

	values := []any{
		headerToArray(zone.RateLimitHeader),
	}

	for _, entry := range zone.RateLimitEntries {
		values = append(values, entryToArray(entry))
	}

	if err := encoder.Encode(values); err != nil {
		return fmt.Errorf("error encoding entries: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, endpoint, &buf)
	if err != nil {
		return fmt.Errorf("error creating request: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-msgpack")

	resp, err := w.client.Do(req)
	if err != nil {
		return fmt.Errorf("error sending request to %s: %w", endpoint, err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code %d from %s", resp.StatusCode, endpoint)
	}

	defer func() {
		if err := resp.Body.Close(); err != nil {
			w.logger.Error("Error closing response body", "error", err)
		}
	}()
	return nil
}

func headerToArray(header ratelimit.RateLimitHeader) []any {
	return []any{
		header.Key,
		header.Now,
		header.NowMonotonic,
	}
}

func entryToArray(entry ratelimit.RateLimitEntry) []any {
	return []any{
		entry.Key,
		entry.Last,
		entry.Excess,
	}
}
