package docker

import (
	"net/http"
	"sync"
	"testing"
)

func TestProbeServerVersionRace(t *testing.T) {
	t.Parallel()
	client, cleanup := newHTTPTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/version" {
			w.Write([]byte(`{"ApiVersion":"1.44"}`))
			return
		}
		w.WriteHeader(http.StatusOK)
	})
	defer cleanup()
	client.SkipServerVersionCheck = false // Trigger the logic

	var wg sync.WaitGroup
	const workers = 100
	errs := make(chan error, workers)
	wg.Add(workers)
	for i := 0; i < workers; i++ {
		go func() {
			defer wg.Done()
			errs <- client.Ping()
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
}
