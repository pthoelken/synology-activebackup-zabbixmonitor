package zabbix

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"

	"github.com/pthoelken/synology-activebackup-zabbixmonitor/internal/collector"
)

type Discovery struct {
	Data []DiscoveryEntry `json:"data"`
}

type DiscoveryEntry struct {
	Product     string `json:"{#PRODUCT}"`
	TaskID      string `json:"{#TASKID}"`
	JobName     string `json:"{#JOBNAME}"`
	ServiceType string `json:"{#SERVICETYPE}"`
	BackupType  string `json:"{#BACKUPTYPE}"`
}

func DiscoveryJSON(snapshot collector.Snapshot, product string) ([]byte, error) {
	entries := make([]DiscoveryEntry, 0, len(snapshot.Jobs))
	for _, job := range snapshot.Jobs {
		if product != "" && job.Product != product {
			continue
		}
		if !job.HasData {
			continue
		}
		entries = append(entries, DiscoveryEntry{
			Product:     job.Product,
			TaskID:      job.TaskID,
			JobName:     job.JobName,
			ServiceType: job.ServiceType,
			BackupType:  job.BackupType,
		})
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].Product == entries[j].Product {
			return entries[i].TaskID < entries[j].TaskID
		}
		return entries[i].Product < entries[j].Product
	})
	return json.Marshal(Discovery{Data: entries})
}

func FindJob(snapshot collector.Snapshot, product string, taskID string) (collector.Job, bool) {
	for _, job := range snapshot.Jobs {
		if job.Product == product && job.TaskID == taskID {
			return job, true
		}
	}
	return collector.Job{}, false
}

func JobField(job collector.Job, field string) (string, error) {
	switch field {
	case "status":
		return strconv.Itoa(job.Status), nil
	case "age":
		return strconv.FormatInt(job.AgeSeconds, 10), nil
	case "last_success_age":
		if job.LastSuccessTime == nil {
			return strconv.Itoa(collector.StatusNoData), nil
		}
		return strconv.FormatInt(job.LastSuccessAgeSeconds, 10), nil
	case "error":
		if job.ErrorCode == "" {
			return "0", nil
		}
		return job.ErrorCode, nil
	case "runtime":
		return strconv.FormatInt(job.RuntimeSeconds, 10), nil
	case "transferred_size":
		return strconv.FormatInt(job.TransferredSize, 10), nil
	case "last_end_time":
		return strconv.FormatInt(job.LastEndUnix, 10), nil
	case "info":
		data, err := json.Marshal(job)
		return string(data), err
	default:
		return "", fmt.Errorf("unknown job field %q", field)
	}
}

func HealthField(snapshot collector.Snapshot, field string, product string) (string, error) {
	switch field {
	case "", "json":
		data, err := json.Marshal(snapshot.Health)
		return string(data), err
	case "ok":
		if snapshot.Health.OK {
			return "1", nil
		}
		return "0", nil
	case "job_count":
		return strconv.Itoa(snapshot.Health.JobCount), nil
	case "db_missing":
		for _, missing := range snapshot.Health.DBMissing {
			if product == "" || missing == product {
				return "1", nil
			}
		}
		return "0", nil
	default:
		return "", fmt.Errorf("unknown health field %q", field)
	}
}

func SummaryField(snapshot collector.Snapshot, field string) (string, error) {
	var successful int
	var failed int
	var problem int
	for _, job := range snapshot.Jobs {
		switch job.Status {
		case collector.StatusOK:
			successful++
		case collector.StatusFailed:
			failed++
			problem++
		case collector.StatusRunning:
			continue
		default:
			problem++
		}
	}
	switch field {
	case "successful":
		return strconv.Itoa(successful), nil
	case "failed":
		return strconv.Itoa(failed), nil
	case "problem", "warning":
		return strconv.Itoa(problem), nil
	case "total":
		return strconv.Itoa(len(snapshot.Jobs)), nil
	default:
		return "", fmt.Errorf("unknown summary field %q", field)
	}
}
