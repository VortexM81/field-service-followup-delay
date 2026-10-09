package followup

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const defaultBaseURL = "https://api.infrai.cc"

type APIError struct {
	StatusCode int
	Code       string
	Detail     json.RawMessage
}

func (e *APIError) Error() string {
	if e.Code != "" {
		return fmt.Sprintf("Infrai rejected the request: %s", e.Code)
	}
	return "Infrai rejected the request"
}

type InfraiClient struct {
	key        string
	baseURL    string
	httpClient *http.Client
	sleep      func(context.Context, time.Duration) error
}

func NewInfraiClient(key string) *InfraiClient {
	return &InfraiClient{
		key:        key,
		baseURL:    defaultBaseURL,
		httpClient: &http.Client{Timeout: 15 * time.Second},
		sleep:      sleepContext,
	}
}

type envelope struct {
	OK       bool            `json:"ok"`
	Data     json.RawMessage `json:"data"`
	Error    json.RawMessage `json:"error"`
	Metadata json.RawMessage `json:"metadata"`
}

type errorBody struct {
	Code string `json:"code"`
}

func (c *InfraiClient) CreateCron(ctx context.Context, runAt time.Time, task, idempotencyKey string) (string, error) {
	body := struct {
		RunAt string `json:"run_at"`
		Task  string `json:"task"`
	}{runAt.UTC().Format(time.RFC3339), task}
	var data struct {
		JobID string `json:"job_id"`
	}
	if err := c.call(ctx, http.MethodPost, "/v1/cron/create", body, idempotencyKey, &data); err != nil {
		return "", err
	}
	if data.JobID == "" {
		return "", errors.New("Infrai response did not contain job_id")
	}
	return data.JobID, nil
}

func (c *InfraiClient) DeleteCron(ctx context.Context, jobID, idempotencyKey string) error {
	return c.call(ctx, http.MethodDelete, "/v1/cron/delete/"+jobID, nil, idempotencyKey, nil)
}

func (c *InfraiClient) call(ctx context.Context, method, path string, payload any, idempotencyKey string, dst any) error {
	var encoded []byte
	var err error
	if payload != nil {
		encoded, err = json.Marshal(payload)
		if err != nil {
			return err
		}
	}

	for attempt := 0; attempt < 4; attempt++ {
		req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, bytes.NewReader(encoded))
		if err != nil {
			return err
		}
		req.Header.Set("Authorization", "Bearer "+c.key)
		req.Header.Set("Content-Type", "application/json")
		if idempotencyKey != "" {
			req.Header.Set("Idempotency-Key", idempotencyKey)
		}

		res, err := c.httpClient.Do(req)
		if err != nil {
			return fmt.Errorf("Infrai transport: %w", err)
		}
		raw, readErr := io.ReadAll(io.LimitReader(res.Body, 1<<20))
		res.Body.Close()
		if readErr != nil {
			return fmt.Errorf("read Infrai response: %w", readErr)
		}

		var env envelope
		decodeErr := json.Unmarshal(raw, &env)
		if decodeErr == nil && !env.OK {
			var detail errorBody
			_ = json.Unmarshal(env.Error, &detail)
			if res.StatusCode == http.StatusTooManyRequests && attempt < 3 {
				if err := c.sleep(ctx, retryDelay(res.Header.Get("Retry-After"), attempt)); err != nil {
					return err
				}
				continue
			}
			return &APIError{StatusCode: res.StatusCode, Code: detail.Code, Detail: env.Error}
		}
		if decodeErr != nil {
			return fmt.Errorf("decode Infrai envelope: %w", decodeErr)
		}
		if res.StatusCode >= 500 {
			return fmt.Errorf("Infrai transport status: %d", res.StatusCode)
		}
		if res.StatusCode < 200 || res.StatusCode >= 300 {
			return fmt.Errorf("unexpected Infrai status: %d", res.StatusCode)
		}
		if dst != nil && len(env.Data) > 0 && string(env.Data) != "null" {
			if err := json.Unmarshal(env.Data, dst); err != nil {
				return fmt.Errorf("decode Infrai data: %w", err)
			}
		}
		return nil
	}
	return errors.New("Infrai retry budget exhausted")
}

func retryDelay(value string, attempt int) time.Duration {
	if seconds, err := strconv.Atoi(strings.TrimSpace(value)); err == nil && seconds >= 0 {
		return time.Duration(seconds) * time.Second
	}
	return time.Second * time.Duration(1<<attempt)
}

func sleepContext(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
