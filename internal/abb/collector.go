package abb

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"time"

	"github.com/pthoelken/synology-activebackup-zabbixmonitor/internal/collector"
	"github.com/pthoelken/synology-activebackup-zabbixmonitor/internal/synology"
)

type Collector struct {
	ScanPaths []string
	Logger    *slog.Logger
}

type schemaCandidate struct {
	Table       string
	TaskIDCol   string
	NameCol     string
	StatusCol   string
	StartCol    string
	EndCol      string
	ErrorCol    string
	TypeCol     string
	ServiceCol  string
	SizeCol     string
	Columns     []string
	ColumnIndex map[string]int
}

type genericRun struct {
	TaskID          string
	Name            string
	Status          string
	ErrorCode       string
	Start           *time.Time
	End             *time.Time
	BackupType      string
	ServiceType     string
	TransferredSize int64
	SourceDB        string
	Table           string
}

func (c Collector) Collect(ctx context.Context, now time.Time) collector.Result {
	var result collector.Result
	dbs := synology.FindSQLiteDBs(c.ScanPaths)
	if len(dbs) == 0 {
		result.Sources = append(result.Sources, collector.Source{
			Product: collector.ProductABB,
			Path:    strings.Join(c.ScanPaths, ","),
			Kind:    "sqlite",
			Found:   false,
			Error:   "database not found",
		})
		return result
	}

	for _, path := range dbs {
		source := collector.Source{Product: collector.ProductABB, Path: path, Kind: "sqlite", Found: true}
		db, err := synology.OpenSQLiteReadOnly(path)
		if err != nil {
			source.Error = err.Error()
			result.Sources = append(result.Sources, source)
			result.Errors = append(result.Errors, fmt.Errorf("abb %s: %w", path, err))
			continue
		}
		jobs, err := c.collectDB(ctx, db, path, now)
		_ = db.Close()
		if err != nil {
			source.Error = err.Error()
			result.Errors = append(result.Errors, fmt.Errorf("abb %s: %w", path, err))
		}
		result.Sources = append(result.Sources, source)
		result.Jobs = mergeJobs(result.Jobs, jobs)
	}
	return result
}

func (c Collector) collectDB(ctx context.Context, db *sql.DB, path string, now time.Time) ([]collector.Job, error) {
	tables, err := synology.ListTables(db)
	if err != nil {
		return nil, err
	}
	var candidates []schemaCandidate
	for _, table := range tables {
		if candidate, ok := newCandidate(table); ok {
			candidates = append(candidates, candidate)
		}
	}
	if len(candidates) == 0 {
		return nil, fmt.Errorf("no supported ABB activity/log schema found")
	}

	grouped := map[string][]genericRun{}
	for _, candidate := range candidates {
		runs, err := c.readCandidate(ctx, db, path, candidate)
		if err != nil {
			continue
		}
		for _, run := range runs {
			if run.TaskID == "" {
				continue
			}
			grouped[run.TaskID] = append(grouped[run.TaskID], run)
		}
	}
	if len(grouped) == 0 {
		return nil, nil
	}

	var jobs []collector.Job
	for taskID, runs := range grouped {
		latest := latestGenericRun(runs)
		success := latestSuccessfulGenericRun(runs)
		status := collector.StatusFromRaw(collector.ProductABB, strings.ToLower(strings.TrimSpace(latest.Status)))
		job := collector.Job{
			Product:         collector.ProductABB,
			TaskID:          taskID,
			JobName:         jobName(taskID, latest),
			ServiceType:     latest.ServiceType,
			BackupType:      latest.BackupType,
			Status:          status,
			RawStatus:       latest.Status,
			ErrorCode:       latest.ErrorCode,
			StartTime:       latest.Start,
			EndTime:         latest.End,
			LastSuccessTime: success.End,
			RuntimeSeconds:  synology.RuntimeSeconds(latest.Start, latest.End),
			TransferredSize: latest.TransferredSize,
			HasData:         true,
			SourceDB:        latest.SourceDB,
			Info: map[string]string{
				"table":  latest.Table,
				"status": collector.StatusName(status),
			},
		}
		if latest.End != nil {
			job.LastEndUnix = latest.End.Unix()
			job.AgeSeconds = synology.AgeSeconds(now, latest.End)
		}
		if success.End != nil {
			job.LastSuccessAgeSeconds = synology.AgeSeconds(now, success.End)
		}
		jobs = append(jobs, job)
	}
	sort.Slice(jobs, func(i, j int) bool {
		return jobs[i].TaskID < jobs[j].TaskID
	})
	return jobs, nil
}

func newCandidate(table synology.TableInfo) (schemaCandidate, bool) {
	lowerTable := strings.ToLower(table.Name)
	if !strings.Contains(lowerTable, "activity") &&
		!strings.Contains(lowerTable, "log") &&
		!strings.Contains(lowerTable, "history") &&
		!strings.Contains(lowerTable, "task") &&
		!strings.Contains(lowerTable, "job") &&
		!strings.Contains(lowerTable, "execution") {
		return schemaCandidate{}, false
	}

	columnMap := synology.ColumnNames(table.Columns)
	candidate := schemaCandidate{
		Table:       table.Name,
		TaskIDCol:   choose(columnMap, "task_id", "taskid", "job_id", "jobid", "task_uuid", "task_guid", "uuid", "id"),
		NameCol:     choose(columnMap, "task_name", "job_name", "name", "title"),
		StatusCol:   choose(columnMap, "execution_status", "status", "result_status", "result", "state", "state_code"),
		StartCol:    choose(columnMap, "start_run_time", "start_time", "begin_time", "started_at", "time_start", "create_time", "created_at"),
		EndCol:      choose(columnMap, "end_run_time", "end_time", "finish_time", "finished_at", "completed_at", "complete_time", "update_time", "updated_at"),
		ErrorCol:    choose(columnMap, "error_code", "error", "error_id", "fail_code", "result_code"),
		TypeCol:     choose(columnMap, "job_type", "backup_type", "type"),
		ServiceCol:  choose(columnMap, "service_type", "service", "category"),
		SizeCol:     choose(columnMap, "transferred_size", "transfer_size", "data_size", "bytes", "size"),
		ColumnIndex: map[string]int{},
	}
	for _, col := range table.Columns {
		candidate.Columns = append(candidate.Columns, col.Name)
	}
	sort.Strings(candidate.Columns)
	for idx, col := range candidate.Columns {
		candidate.ColumnIndex[col] = idx
	}

	if candidate.TaskIDCol == "" && candidate.NameCol == "" {
		return schemaCandidate{}, false
	}
	if candidate.StatusCol == "" && candidate.StartCol == "" && candidate.EndCol == "" {
		return schemaCandidate{}, false
	}
	return candidate, true
}

func (c Collector) readCandidate(ctx context.Context, db *sql.DB, path string, candidate schemaCandidate) ([]genericRun, error) {
	query := "SELECT " + quoteColumns(candidate.Columns) + " FROM " + synology.QuoteIdent(candidate.Table) + " LIMIT 5000"
	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var runs []genericRun
	for rows.Next() {
		values := make([]any, len(candidate.Columns))
		dest := make([]any, len(values))
		for i := range values {
			dest[i] = &values[i]
		}
		if err := rows.Scan(dest...); err != nil {
			return nil, err
		}
		get := func(col string) any {
			if col == "" {
				return nil
			}
			idx, ok := candidate.ColumnIndex[col]
			if !ok {
				return nil
			}
			return values[idx]
		}
		taskID := synology.StringValue(get(candidate.TaskIDCol))
		name := synology.StringValue(get(candidate.NameCol))
		if taskID == "" {
			taskID = name
		}
		run := genericRun{
			TaskID:          taskID,
			Name:            name,
			Status:          synology.StringValue(get(candidate.StatusCol)),
			ErrorCode:       synology.StringValue(get(candidate.ErrorCol)),
			Start:           synology.ParseTimeValue(get(candidate.StartCol)),
			End:             synology.ParseTimeValue(get(candidate.EndCol)),
			BackupType:      synology.StringValue(get(candidate.TypeCol)),
			ServiceType:     synology.StringValue(get(candidate.ServiceCol)),
			TransferredSize: synology.Int64Value(get(candidate.SizeCol)),
			SourceDB:        path,
			Table:           candidate.Table,
		}
		if run.TaskID == "" || (run.Status == "" && run.Start == nil && run.End == nil) {
			continue
		}
		runs = append(runs, run)
	}
	return runs, rows.Err()
}

func choose(columnMap map[string]string, names ...string) string {
	for _, name := range names {
		if original, ok := columnMap[name]; ok {
			return original
		}
	}
	return ""
}

func quoteColumns(columns []string) string {
	quoted := make([]string, 0, len(columns))
	for _, col := range columns {
		quoted = append(quoted, synology.QuoteIdent(col))
	}
	return strings.Join(quoted, ", ")
}

func latestGenericRun(runs []genericRun) genericRun {
	sort.SliceStable(runs, func(i, j int) bool {
		return genericRunTime(runs[i]).After(genericRunTime(runs[j]))
	})
	if len(runs) == 0 {
		return genericRun{}
	}
	return runs[0]
}

func latestSuccessfulGenericRun(runs []genericRun) genericRun {
	var matches []genericRun
	for _, r := range runs {
		if collector.StatusFromRaw(collector.ProductABB, strings.ToLower(strings.TrimSpace(r.Status))) == collector.StatusOK {
			matches = append(matches, r)
		}
	}
	if len(matches) == 0 {
		return genericRun{}
	}
	return latestGenericRun(matches)
}

func genericRunTime(r genericRun) time.Time {
	if r.End != nil {
		return *r.End
	}
	if r.Start != nil {
		return *r.Start
	}
	return time.Unix(0, 0)
}

func jobName(taskID string, run genericRun) string {
	if run.Name != "" {
		return run.Name
	}
	return "ABB task " + taskID
}

func mergeJobs(existing []collector.Job, incoming []collector.Job) []collector.Job {
	index := map[string]int{}
	for i, job := range existing {
		index[job.Product+"/"+job.TaskID] = i
	}
	for _, job := range incoming {
		key := job.Product + "/" + job.TaskID
		i, ok := index[key]
		if !ok {
			existing = append(existing, job)
			index[key] = len(existing) - 1
			continue
		}
		if job.EndTime != nil && (existing[i].EndTime == nil || job.EndTime.After(*existing[i].EndTime)) {
			existing[i] = job
		}
	}
	return existing
}
