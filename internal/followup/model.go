package followup

import (
	"errors"
	"time"
)

type DispatchStatus string

const (
	DispatchPending    DispatchStatus = "pending"
	DispatchDispatched DispatchStatus = "dispatched"
)

type WorkOrder struct {
	ID             string         `json:"id"`
	PhotoURLs      []string       `json:"photo_urls"`
	DispatchStatus DispatchStatus `json:"dispatch_status"`
}

type FollowUpRequest struct {
	WorkOrder  WorkOrder `json:"work_order"`
	DelayHours int       `json:"delay_hours"`
}

type ScheduledFollowUp struct {
	WorkOrderID string    `json:"work_order_id"`
	RunAt       time.Time `json:"run_at"`
	JobID       string    `json:"job_id"`
	Token       string    `json:"token"`
}

func DecideFollowUp(req FollowUpRequest, now time.Time) (time.Time, error) {
	if req.WorkOrder.ID == "" {
		return time.Time{}, errors.New("work_order.id is required")
	}
	if req.WorkOrder.DispatchStatus != DispatchDispatched {
		return time.Time{}, errors.New("work order must be dispatched")
	}
	if len(req.WorkOrder.PhotoURLs) == 0 {
		return time.Time{}, errors.New("at least one work-order photo is required")
	}
	if req.DelayHours < 1 || req.DelayHours > 168 {
		return time.Time{}, errors.New("delay_hours must be between 1 and 168")
	}
	return now.UTC().Add(time.Duration(req.DelayHours) * time.Hour), nil
}
