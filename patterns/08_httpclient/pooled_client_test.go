package httpclient_test

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	httpclient "system-design-patterns/patterns/08_httpclient"
)

func TestPooledHTTPClient_ReuseUnderLoad(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}))
	defer ts.Close()

	cfg := httpclient.DefaultPooledClientConfig()
	cfg.OverallTimeout = 2 * time.Second
	client := httpclient.NewPooledHTTPClient(cfg)

	var wg sync.WaitGroup
	for range 20 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range 5 {
				resp, err := client.Get(ts.URL)
				if err != nil {
					t.Errorf("request failed: %v", err)
					return
				}
				_ = resp.Body.Close()
			}
		}()
	}

	wg.Wait()
}
