package followup

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type CronScheduler interface {
	CreateCron(context.Context, time.Time, string, string) (string, error)
	DeleteCron(context.Context, string, string) error
}

type Service struct {
	scheduler CronScheduler
	store     *Store
	publicURL string
	now       func() time.Time
	logger    *slog.Logger
}

func NewService(scheduler CronScheduler, store *Store, publicURL string, logger *slog.Logger) *Service {
	return &Service{scheduler: scheduler, store: store, publicURL: strings.TrimRight(publicURL, "/"), now: time.Now, logger: logger}
}

func (s *Service) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /follow-ups", s.schedule)
	mux.HandleFunc("POST /callbacks/follow-up", s.run)
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
	return mux
}

func (s *Service) schedule(w http.ResponseWriter, r *http.Request) {
	var req FollowUpRequest
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	runAt, err := DecideFollowUp(req, s.now())
	if err != nil {
		writeJSON(w, http.StatusUnprocessableEntity, map[string]string{"error": err.Error()})
		return
	}
	token, err := randomToken()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "could not allocate follow-up"})
		return
	}
	callback := s.publicURL + "/callbacks/follow-up?token=" + url.QueryEscape(token)
	jobID, err := s.scheduler.CreateCron(r.Context(), runAt, callback, "follow-up:"+req.WorkOrder.ID+":"+runAt.Format(time.RFC3339))
	if err != nil {
		s.writeSchedulerError(w, err)
		return
	}
	item := ScheduledFollowUp{WorkOrderID: req.WorkOrder.ID, RunAt: runAt, JobID: jobID, Token: token}
	if err := s.store.Put(item); err != nil {
		s.logger.Error("persist follow-up", "work_order_id", req.WorkOrder.ID, "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "could not persist follow-up"})
		return
	}
	s.logger.Info("follow-up scheduled", "work_order_id", item.WorkOrderID, "run_at", item.RunAt, "job_id", item.JobID)
	writeJSON(w, http.StatusCreated, item)
}

func (s *Service) run(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	item, ok, err := s.store.Take(token)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "could not update follow-up"})
		return
	}
	if !ok {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if err := s.scheduler.DeleteCron(r.Context(), item.JobID, "complete:"+item.JobID); err != nil {
		s.writeSchedulerError(w, err)
		return
	}
	s.logger.Info("technician follow-up due", "work_order_id", item.WorkOrderID, "job_id", item.JobID)
	writeJSON(w, http.StatusOK, map[string]string{"work_order_id": item.WorkOrderID, "status": "follow_up_due"})
}

func (s *Service) writeSchedulerError(w http.ResponseWriter, err error) {
	var apiErr *APIError
	if errors.As(err, &apiErr) && apiErr.StatusCode >= 400 && apiErr.StatusCode < 500 {
		writeJSON(w, apiErr.StatusCode, map[string]string{"error": apiErr.Error()})
		return
	}
	s.logger.Error("scheduler request", "error", err)
	writeJSON(w, http.StatusBadGateway, map[string]string{"error": "scheduler request failed"})
}

func randomToken() (string, error) {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", fmt.Errorf("random token: %w", err)
	}
	return hex.EncodeToString(raw[:]), nil
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
