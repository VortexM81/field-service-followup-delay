package followup

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestCreateCronRetriesRateLimitAndReadsEnvelope(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Errorf("missing bearer authorization")
		}
		if r.Header.Get("Idempotency-Key") != "follow-up:WO-1042" {
			t.Errorf("missing idempotency key")
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if got := body["run_at"]; got != "2026-09-05T14:30:00Z" {
			t.Errorf("run_at = %v, want 2026-09-05T14:30:00Z", got)
		}
		if _, ok := body["cron_expr"]; ok {
			t.Error("one-shot request unexpectedly contains cron_expr")
		}
		w.Header().Set("Content-Type", "application/json")
		if attempts == 1 {
			w.Header().Set("Retry-After", "2")
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`{"ok":false,"data":null,"error":{"code":"RATE_LIMITED"},"metadata":{}}`))
			return
		}
		_, _ = w.Write([]byte(`{"ok":true,"data":{"job_id":"job-42"},"error":null,"metadata":{}}`))
	}))
	defer server.Close()

	client := NewInfraiClient("test-key")
	client.baseURL = server.URL
	client.sleep = func(_ context.Context, delay time.Duration) error {
		if delay != 2*time.Second {
			t.Fatalf("retry delay = %v, want 2s", delay)
		}
		return nil
	}
	runAt := time.Date(2026, time.September, 5, 14, 30, 0, 0, time.UTC)
	jobID, err := client.CreateCron(context.Background(), runAt, "https://service.example/callback", "follow-up:WO-1042")
	if err != nil {
		t.Fatal(err)
	}
	if jobID != "job-42" || attempts != 2 {
		t.Fatalf("jobID = %q, attempts = %d", jobID, attempts)
	}
}
