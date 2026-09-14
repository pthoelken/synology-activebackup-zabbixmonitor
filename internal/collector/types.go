package collector

import (
	"strings"
	"time"
)

const (
	ProductABB  = "abb"
	ProductM365 = "m365"

	StatusOK        = 1
	StatusWarning   = 2
	StatusRunning   = 3
	StatusFailed    = 6
	StatusNoData    = 8
	StatusDBMissing = 9
	StatusUnknown   = 10
)

type Job struct {
	Product               string            `json:"product"`
	TaskID                string            `json:"task_id"`
	JobName               string            `json:"job_name"`
	ServiceType           string            `json:"service_type,omitempty"`
	BackupType            string            `json:"backup_type,omitempty"`
	Status                int               `json:"status"`
	RawStatus             string            `json:"raw_status,omitempty"`
	ErrorCode             string            `json:"error_code,omitempty"`
	StartTime             *time.Time        `json:"start_time,omitempty"`
	EndTime               *time.Time        `json:"end_time,omitempty"`
	LastSuccessTime       *time.Time        `json:"last_success_time,omitempty"`
	LastEndUnix           int64             `json:"last_end_unix"`
	AgeSeconds            int64             `json:"age_seconds"`
	LastSuccessAgeSeconds int64             `json:"last_success_age_seconds"`
	RuntimeSeconds        int64             `json:"runtime_seconds"`
	TransferredSize       int64             `json:"transferred_size"`
	HasData               bool              `json:"has_data"`
	SourceDB              string            `json:"source_db,omitempty"`
	Info                  map[string]string `json:"info,omitempty"`
}

type Source struct {
	Product string `json:"product"`
	Path    string `json:"path"`
	Kind    string `json:"kind"`
	Found   bool   `json:"found"`
	Error   string `json:"error,omitempty"`
}

type Health struct {
	OK              bool     `json:"ok"`
	CollectorErrors []string `json:"collector_errors,omitempty"`
	DBMissing       []string `json:"db_missing,omitempty"`
	JobCount        int      `json:"job_count"`
	CollectedUnix   int64    `json:"collected_unix"`
}

type Snapshot struct {
	CollectedAt time.Time `json:"collected_at"`
	Health      Health    `json:"health"`
	Jobs        []Job     `json:"jobs"`
	Sources     []Source  `json:"sources"`
	Errors      []string  `json:"errors,omitempty"`
}

type Result struct {
	Jobs    []Job
	Sources []Source
	Errors  []error
}

func StatusFromRaw(product string, raw string) int {
	raw = strings.TrimSpace(strings.ToLower(raw))
	switch product {
	case ProductABB:
		switch raw {
		case "2", "successful", "success", "succeeded", "ok", "completed", "complete", "finished", "done":
			return StatusOK
		case "1", "3", "5", "8", "incomplete", "partial successful", "partial_successful", "partial success", "partial_success", "partial", "partially completed", "not fully completed", "completed with warnings", "canceled", "cancelled":
			return StatusWarning
		case "4", "failed", "failure", "error", "aborted":
			return StatusFailed
		case "running", "in_progress", "processing":
			return StatusRunning
		case "", "0", "none":
			return StatusNoData
		default:
			return StatusUnknown
		}
	case ProductM365:
		switch raw {
		case "1", "successful", "success", "succeeded", "ok", "completed", "complete", "finished", "done":
			return StatusOK
		case "6", "warning", "partial", "partial successful", "partial_successful", "skipped", "completed with skipped items":
			return StatusWarning
		case "4", "failed", "failure", "error", "aborted", "cancelled", "canceled":
			return StatusFailed
		case "running", "in_progress", "processing", "preparing":
			return StatusRunning
		case "", "0", "none":
			return StatusNoData
		default:
			return StatusUnknown
		}
	default:
		switch raw {
		case "1", "success", "succeeded", "successful", "ok", "completed", "complete", "finished", "done":
			return StatusOK
		case "6", "failed", "failure", "error", "aborted", "cancelled", "canceled":
			return StatusFailed
		default:
			return StatusUnknown
		}
	}
}

func StatusName(status int) string {
	switch status {
	case StatusOK:
		return "OK"
	case StatusWarning:
		return "Warning"
	case StatusRunning:
		return "Running"
	case StatusFailed:
		return "Failed"
	case StatusNoData:
		return "No data"
	case StatusDBMissing:
		return "DB missing"
	default:
		return "Unknown"
	}
}
