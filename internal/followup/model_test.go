package followup

import (
	"testing"
	"time"
)

func TestDecideFollowUp(t *testing.T) {
	now := time.Date(2026, time.September, 5, 8, 30, 0, 0, time.UTC)
	tests := []struct {
		name    string
		req     FollowUpRequest
		want    time.Time
		wantErr bool
	}{
		{
			name: "dispatched order with photo is scheduled",
			req:  FollowUpRequest{WorkOrder: WorkOrder{ID: "WO-1042", PhotoURLs: []string{"https://photos.example/1042/after.jpg"}, DispatchStatus: DispatchDispatched}, DelayHours: 6},
			want: now.Add(6 * time.Hour),
		},
		{
			name:    "pending dispatch is rejected",
			req:     FollowUpRequest{WorkOrder: WorkOrder{ID: "WO-1042", PhotoURLs: []string{"https://photos.example/1042/after.jpg"}, DispatchStatus: DispatchPending}, DelayHours: 6},
			wantErr: true,
		},
		{
			name:    "missing photo is rejected",
			req:     FollowUpRequest{WorkOrder: WorkOrder{ID: "WO-1042", DispatchStatus: DispatchDispatched}, DelayHours: 6},
			wantErr: true,
		},
		{
			name:    "delay beyond seven days is rejected",
			req:     FollowUpRequest{WorkOrder: WorkOrder{ID: "WO-1042", PhotoURLs: []string{"https://photos.example/1042/after.jpg"}, DispatchStatus: DispatchDispatched}, DelayHours: 169},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := DecideFollowUp(tt.req, now)
			if (err != nil) != tt.wantErr {
				t.Fatalf("DecideFollowUp() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && !got.Equal(tt.want) {
				t.Fatalf("DecideFollowUp() = %v, want %v", got, tt.want)
			}
		})
	}
}
