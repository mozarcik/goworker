package goworker

import "time"

const (
	STATUS_WAITING   = "waiting"
	STATUS_RUNNING   = "running"
	STATUS_FAILED    = "failed"
	STATUS_COMPLETED = "completed"
)

type JobStatus struct {
	Status    string    `json:"status"`
	UpdatedAt time.Time `json:"updated"`
	StartedAt time.Time `json:"started"`
}
