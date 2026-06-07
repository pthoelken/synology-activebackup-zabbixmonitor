package m365

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"time"

	"github.com/pthoelken/synology-activebackup-zabbixmonitor/internal/collector"
	"github.com/pthoelken/synology-activebackup-zabbixmonitor/internal/logging"
	"github.com/pthoelken/synology-activebackup-zabbixmonitor/internal/synology"
)

type Collector struct {
	ScanPaths   []string
	RedactNames bool
	Logger      *slog.Logger
}

type run struct {
	TaskID          string
	TaskExecutionID string
	Status          string
	ErrorCode       string
	Start           *time.Time
	End             *time.Time
	JobType         string
	ServiceType     string
	SelectedItem    string
	UserName        string
	ToUserName      string
	TransferredSize int64
	SourceDB        string
}

func (c Collector) Collect(ctx context.Context, now time.Time) collector.Result {
	var result collector.Result
	dbs := synology.FindM365LogDBs(c.ScanPaths)
	if len(dbs) == 0 {
		result.Sources = append(result.Sources, collector.Source{
			Product: collector.ProductM365,
			Path:    strings.Join(c.ScanPaths, ","),
			Kind:    "log.sqlite",
			Found:   false,
			Error:   "database not found",
		})
		return result
	}

	for _, path := range dbs {
		source := collector.Source{Product: collector.ProductM365, Path: path, Kind: "log.sqlite", Found: true}
		db, err := synology.OpenSQLiteReadOnly(path)
		if err != nil {
			source.Error = err.Error()
			result.Sources = append(result.Sources, source)
			result.Errors = append(result.Errors, fmt.Errorf("m365 %s: %w", path, err))
			continue
		}
		jobs, err := c.collectDB(ctx, db, path, now)
		_ = db.Close()
		if err != nil {
			source.Error = err.Error()
			result.Errors = append(result.Errors, fmt.Errorf("m365 %s: %w", path, err))
		}
		result.Sources = append(result.Sources, source)
		result.Jobs = append(result.Jobs, jobs...)
	}

	return result
}

func (c Collector) collectDB(ctx context.Context, db *sql.DB, path string, now time.Time) ([]collector.Job, error) {
	tables, err := synology.ListTables(db)
	if err != nil {
		return nil, err
	}
	if !synology.HasTable(tables, "job_log_table") {
		return nil, fmt.Errorf("job_log_table not found")
	}

	query := `
SELECT task_id, task_execution_id, execution_status, error_code, start_run_time, end_run_time,
       job_type, service_type, selected_item, user_name, to_user_name, transferred_size
FROM job_log_table`
	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	grouped := map[string][]run{}
	for rows.Next() {
		values := make([]any, 12)
		dest := make([]any, len(values))
		for i := range values {
			dest[i] = &values[i]
		}
		if err := rows.Scan(dest...); err != nil {
			return nil, err
		}
		r := run{
			TaskID:          synology.StringValue(values[0]),
			TaskExecutionID: synology.StringValue(values[1]),
			Status:          synology.StringValue(values[2]),
			ErrorCode:       synology.StringValue(values[3]),
			Start:           synology.ParseTimeValue(values[4]),
			End:             synology.ParseTimeValue(values[5]),
			JobType:         synology.StringValue(values[6]),
			ServiceType:     synology.StringValue(values[7]),
			SelectedItem:    synology.StringValue(values[8]),
			UserName:        synology.StringValue(values[9]),
			ToUserName:      synology.StringValue(values[10]),
			TransferredSize: synology.Int64Value(values[11]),
			SourceDB:        path,
		}
		if r.TaskID == "" {
			continue
		}
		grouped[r.TaskID] = append(grouped[r.TaskID], r)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	var jobs []collector.Job
	for taskID, runs := range grouped {
		latest := latestRun(runs)
		success := latestSuccessfulRun(runs)
		job := collector.Job{
			Product:         collector.ProductM365,
			TaskID:          taskID,
			JobName:         c.jobName(taskID, latest),
			ServiceType:     latest.ServiceType,
			BackupType:      latest.JobType,
			Status:          collector.StatusFromRaw(collector.ProductM365, strings.ToLower(strings.TrimSpace(latest.Status))),
			RawStatus:       latest.Status,
			ErrorCode:       latest.ErrorCode,
			StartTime:       latest.Start,
			EndTime:         latest.End,
			LastSuccessTime: success.End,
			RuntimeSeconds:  synology.RuntimeSeconds(latest.Start, latest.End),
			TransferredSize: latest.TransferredSize,
			HasData:         true,
			SourceDB:        path,
			Info: map[string]string{
				"task_execution_id": latest.TaskExecutionID,
				"status":            collector.StatusName(collector.StatusFromRaw(collector.ProductM365, strings.ToLower(strings.TrimSpace(latest.Status)))),
			},
		}
		if latest.End != nil {
			job.LastEndUnix = latest.End.Unix()
			job.AgeSeconds = synology.AgeSeconds(now, latest.End)
		}
		if success.End != nil {
			job.LastSuccessAgeSeconds = synology.AgeSeconds(now, success.End)
		}
		if latest.ServiceType != "" {
			job.Info["service_type"] = latest.ServiceType
		}
		if latest.JobType != "" {
			job.Info["job_type"] = latest.JobType
		}
		if !c.RedactNames && latest.SelectedItem != "" {
			job.Info["selected_item"] = latest.SelectedItem
		}
		jobs = append(jobs, job)
	}
	sort.Slice(jobs, func(i, j int) bool {
		return jobs[i].TaskID < jobs[j].TaskID
	})
	return jobs, nil
}

func (c Collector) jobName(taskID string, r run) string {
	if !c.RedactNames && r.SelectedItem != "" {
		return r.SelectedItem
	}
	if !c.RedactNames && r.UserName != "" {
		return r.UserName
	}
	if c.RedactNames && r.SelectedItem != "" {
		redacted := logging.Redact(r.SelectedItem)
		if redacted != "" && redacted != "***" {
			return "M365 " + redacted
		}
	}
	if r.ServiceType != "" {
		return "M365 " + r.ServiceType + " task " + taskID
	}
	return "M365 task " + taskID
}

func latestRun(runs []run) run {
	sort.SliceStable(runs, func(i, j int) bool {
		return runTime(runs[i]).After(runTime(runs[j]))
	})
	if len(runs) == 0 {
		return run{}
	}
	return runs[0]
}

func latestSuccessfulRun(runs []run) run {
	var matches []run
	for _, r := range runs {
		if collector.StatusFromRaw(collector.ProductM365, strings.ToLower(strings.TrimSpace(r.Status))) == collector.StatusOK {
			matches = append(matches, r)
		}
	}
	if len(matches) == 0 {
		return run{}
	}
	return latestRun(matches)
}

func runTime(r run) time.Time {
	if r.End != nil {
		return *r.End
	}
	if r.Start != nil {
		return *r.Start
	}
	return time.Unix(0, 0)
}
