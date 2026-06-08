package abb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
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

type abbDBSet struct {
	Root     string
	Config   string
	Activity string
	System   string
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

type structuredRun struct {
	DeviceID        string
	TaskID          string
	HostName        string
	TaskName        string
	SourceType      string
	BackupType      string
	Status          string
	Start           *time.Time
	End             *time.Time
	TransferredSize int64
	Speed           string
	Scheduled       string
	Running         bool
	SizeSource      string
	SourceDB        string
}

type structuredSizeTable struct {
	Table       string
	Alias       string
	JSONColumns []string
}

var errUnsupportedSchema = errors.New("no supported ABB activity/log schema found")

func (c Collector) Collect(ctx context.Context, now time.Time) collector.Result {
	var result collector.Result
	sets := findABBDBSets(c.ScanPaths)
	if len(sets) > 0 {
		for _, set := range sets {
			result.Sources = append(result.Sources,
				collector.Source{Product: collector.ProductABB, Path: set.Config, Kind: "config.db", Found: true},
				collector.Source{Product: collector.ProductABB, Path: set.Activity, Kind: "activity.db", Found: true},
			)
			if set.System != "" {
				result.Sources = append(result.Sources, collector.Source{Product: collector.ProductABB, Path: set.System, Kind: "system-db.sqlite", Found: true})
			}
			jobs, err := c.collectStructured(ctx, set, now)
			if err != nil {
				result.Errors = append(result.Errors, fmt.Errorf("abb %s: %w", set.Root, err))
				for i := range result.Sources {
					if result.Sources[i].Path == set.Config || result.Sources[i].Path == set.Activity {
						result.Sources[i].Error = err.Error()
					}
				}
				continue
			}
			result.Jobs = mergeJobs(result.Jobs, jobs)
		}
		return result
	}

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
			if !errors.Is(err, errUnsupportedSchema) {
				result.Errors = append(result.Errors, fmt.Errorf("abb %s: %w", path, err))
			}
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
		return nil, errUnsupportedSchema
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

func findABBDBSets(patterns []string) []abbDBSet {
	seen := map[string]struct{}{}
	var sets []abbDBSet
	for _, path := range synology.ExpandScanPaths(patterns) {
		info, err := os.Stat(path)
		if err != nil {
			continue
		}
		var root string
		if info.IsDir() {
			root = path
		} else {
			switch filepath.Base(path) {
			case "config.db", "activity.db":
				root = filepath.Dir(path)
			case "system-db.sqlite":
				root = filepath.Dir(filepath.Dir(path))
			default:
				continue
			}
		}
		set := abbDBSet{
			Root:     filepath.Clean(root),
			Config:   filepath.Join(root, "config.db"),
			Activity: filepath.Join(root, "activity.db"),
			System:   filepath.Join(root, "agent", "system-db.sqlite"),
		}
		if _, ok := seen[set.Root]; ok {
			continue
		}
		if !regularFile(set.Config) || !regularFile(set.Activity) {
			continue
		}
		if !regularFile(set.System) {
			set.System = ""
		}
		seen[set.Root] = struct{}{}
		sets = append(sets, set)
	}
	sort.Slice(sets, func(i, j int) bool {
		return sets[i].Root < sets[j].Root
	})
	return sets
}

func regularFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func (c Collector) collectStructured(ctx context.Context, set abbDBSet, now time.Time) ([]collector.Job, error) {
	db, err := synology.OpenSQLiteReadOnly(set.Config)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	if err := attachReadOnly(ctx, db, "activity", set.Activity); err != nil {
		return nil, err
	}

	runningIDs := map[string]bool{}
	if set.System != "" {
		runningIDs = readRunningDeviceIDs(ctx, set.System)
	}

	grouped := map[string][]structuredRun{}
	taskIDsWithDevices := map[string]bool{}

	activityTables, err := synology.ListTablesInSchema(db, "activity")
	if err != nil {
		return nil, err
	}
	deviceSizeExpr := structuredABBSizeExpr(activityTables, []structuredSizeTable{
		{Table: "device_result_table", Alias: "drt"},
		{Table: "result_table", Alias: "rt"},
		{Table: "result_detail_table", Alias: "rdt", JSONColumns: []string{"other_params"}},
	})
	taskSizeExpr := structuredABBSizeExpr(activityTables, []structuredSizeTable{
		{Table: "result_table", Alias: "rt"},
		{Table: "result_detail_table", Alias: "rdt", JSONColumns: []string{"other_params"}},
	})
	hasResultDetails := synology.HasTable(activityTables, "result_detail_table")

	deviceRuns, err := readStructuredRuns(ctx, db, structuredABBDeviceQuery(true, deviceSizeExpr, hasResultDetails), set.Activity)
	if err != nil {
		deviceRuns, err = readStructuredRuns(ctx, db, structuredABBDeviceQuery(false, deviceSizeExpr, hasResultDetails), set.Activity)
	}
	if err != nil {
		return nil, err
	}
	for _, run := range deviceRuns {
		if run.DeviceID == "" {
			continue
		}
		grouped[run.DeviceID] = append(grouped[run.DeviceID], run)
		if run.TaskID != "" {
			taskIDsWithDevices[run.TaskID] = true
		}
	}

	if taskRuns, err := readStructuredRuns(ctx, db, structuredABBTaskQuery(taskSizeExpr, hasResultDetails), set.Activity); err == nil {
		for _, run := range taskRuns {
			if run.TaskID == "" || taskIDsWithDevices[run.TaskID] {
				continue
			}
			grouped["task-"+run.TaskID] = append(grouped["task-"+run.TaskID], run)
		}
	}

	var jobs []collector.Job
	for groupID, runs := range grouped {
		run := latestStructuredRun(runs)
		success := latestSuccessfulStructuredRun(runs)
		if run.DeviceID != "" && runningIDs[run.DeviceID] {
			run.Status = "running"
			run.Running = true
		}
		jobID := structuredJobID(groupID, run)
		status := collector.StatusFromRaw(collector.ProductABB, run.Status)
		job := collector.Job{
			Product:         collector.ProductABB,
			TaskID:          jobID,
			JobName:         structuredJobName(run),
			ServiceType:     run.SourceType,
			BackupType:      run.BackupType,
			Status:          status,
			RawStatus:       run.Status,
			StartTime:       run.Start,
			EndTime:         run.End,
			LastSuccessTime: success.End,
			RuntimeSeconds:  synology.RuntimeSeconds(run.Start, run.End),
			TransferredSize: run.TransferredSize,
			HasData:         true,
			SourceDB:        run.SourceDB,
			Info: map[string]string{
				"status": collector.StatusName(status),
			},
		}
		if run.DeviceID != "" {
			job.Info["device_id"] = run.DeviceID
		}
		if run.TaskID != "" {
			job.Info["task_id"] = run.TaskID
		}
		if run.TaskName != "" {
			job.Info["task_name"] = run.TaskName
		}
		if run.Speed != "" {
			job.Info["speed"] = run.Speed
		}
		if run.Scheduled != "" {
			job.Info["scheduled"] = run.Scheduled
		}
		if run.TransferredSize > 0 && run.SizeSource != "" {
			job.Info["transferred_size_source"] = run.SizeSource
		}
		if run.Running {
			job.Info["running"] = "true"
		}
		if run.End != nil {
			job.LastEndUnix = run.End.Unix()
			job.AgeSeconds = synology.AgeSeconds(now, run.End)
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

func readStructuredRuns(ctx context.Context, db *sql.DB, query string, sourceDB string) ([]structuredRun, error) {
	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var runs []structuredRun
	for rows.Next() {
		values := make([]any, 12)
		dest := make([]any, len(values))
		for i := range values {
			dest[i] = &values[i]
		}
		if err := rows.Scan(dest...); err != nil {
			return nil, err
		}
		run := structuredRun{
			HostName:        synology.StringValue(values[0]),
			DeviceID:        synology.StringValue(values[1]),
			TaskID:          synology.StringValue(values[2]),
			TaskName:        synology.StringValue(values[3]),
			SourceType:      synology.StringValue(values[4]),
			BackupType:      synology.StringValue(values[5]),
			Status:          synology.StringValue(values[6]),
			Start:           synology.ParseTimeValue(values[7]),
			End:             synology.ParseTimeValue(values[8]),
			TransferredSize: synology.Int64Value(values[9]),
			Speed:           strings.TrimSpace(synology.StringValue(values[10])),
			Scheduled:       synology.StringValue(values[11]),
			SizeSource:      "activity",
			SourceDB:        sourceDB,
		}
		if run.DeviceID == "" && run.TaskID == "" {
			continue
		}
		runs = append(runs, run)
	}
	return runs, rows.Err()
}

func structuredABBDeviceQuery(strictDeviceMatch bool, sizeExpr string, includeResultDetails bool) string {
	resultJoin := "LEFT JOIN activity.result_table rt ON rt.task_id = btd.task_id"
	if strictDeviceMatch {
		resultJoin += `
	AND (
		rt.task_config IS NULL
		OR rt.task_config = ''
		OR rt.task_config LIKE '%"device_id":' || dt.device_id || ',%'
			OR rt.task_config LIKE '%"device_id": ' || dt.device_id || ',%'
		)`
	}
	detailJoin := ""
	speedExpr := "''"
	detailWhere := ""
	if includeResultDetails {
		detailJoin = "LEFT JOIN activity.result_detail_table rdt ON rt.result_id = rdt.result_id"
		speedExpr = "json_extract(rdt.other_params, '$.speed')"
		detailWhere = "AND (rdt.log_type IS NULL OR rdt.log_type IN ('1111','1102'))"
	}
	return `
	WITH ranked_results AS (
		SELECT
		dt.host_name,
		dt.device_id,
		tt.task_id,
		tt.task_name,
		tt.source_type,
		tt.backup_type,
			drt.status,
			rt.time_start,
			rt.time_end,
			` + sizeExpr + ` AS transferred_size,
			` + speedExpr + ` AS speed,
			json_extract(tt.sched_content, '$.schedule_setting_type') AS scheduled,
			ROW_NUMBER() OVER (PARTITION BY dt.device_id ORDER BY COALESCE(rt.time_start, 0) DESC) AS rn
		FROM device_table dt
	LEFT JOIN backup_task_device btd ON dt.device_id = btd.device_id
		LEFT JOIN task_table tt ON tt.task_id = btd.task_id
		` + resultJoin + `
		LEFT JOIN activity.device_result_table drt ON rt.result_id = drt.result_id
		` + detailJoin + `
		WHERE (rt.job_action IS NULL OR rt.job_action = 1)
		  ` + detailWhere + `
	)
	SELECT host_name, device_id, task_id, task_name, source_type, backup_type, status,
	       time_start, time_end, transferred_size, speed, scheduled
	FROM ranked_results
	WHERE rn <= 50`
}

func structuredABBTaskQuery(sizeExpr string, includeResultDetails bool) string {
	detailJoin := ""
	if includeResultDetails {
		detailJoin = "LEFT JOIN activity.result_detail_table rdt ON rt.result_id = rdt.result_id"
	}
	return `
	WITH ranked_results AS (
	SELECT
		'' AS host_name,
		'' AS device_id,
		tt.task_id,
		tt.task_name,
		tt.source_type,
		tt.backup_type,
			rt.status,
			rt.time_start,
			rt.time_end,
			` + sizeExpr + ` AS transferred_size,
			'' AS speed,
		json_extract(tt.sched_content, '$.schedule_setting_type') AS scheduled,
		ROW_NUMBER() OVER (PARTITION BY tt.task_id ORDER BY COALESCE(rt.time_start, 0) DESC) AS rn
		FROM task_table tt
		LEFT JOIN activity.result_table rt ON rt.task_id = tt.task_id
		` + detailJoin + `
		WHERE (rt.job_action IS NULL OR rt.job_action = 1)
	)
	SELECT host_name, device_id, task_id, task_name, source_type, backup_type, status,
	       time_start, time_end, transferred_size, speed, scheduled
	FROM ranked_results
		WHERE rn <= 50`
}

func structuredABBSizeExpr(tables []synology.TableInfo, specs []structuredSizeTable) string {
	var parts []string
	for _, spec := range specs {
		table, ok := findTableInfo(tables, spec.Table)
		if !ok {
			continue
		}
		if col := chooseSizeColumn(synology.ColumnNames(table.Columns)); col != "" {
			parts = append(parts, fmt.Sprintf("NULLIF(CAST(%s.%s AS INTEGER), 0)", spec.Alias, synology.QuoteIdent(col)))
		}
		columns := synology.ColumnNames(table.Columns)
		for _, jsonCol := range spec.JSONColumns {
			original, ok := columns[synology.NormalizeName(jsonCol)]
			if !ok {
				continue
			}
			for _, key := range sizeJSONKeys() {
				parts = append(parts, fmt.Sprintf(
					"NULLIF(CAST(CASE WHEN json_valid(%[1]s.%[2]s) THEN json_extract(%[1]s.%[2]s, '$.%[3]s') ELSE NULL END AS INTEGER), 0)",
					spec.Alias,
					synology.QuoteIdent(original),
					key,
				))
			}
		}
	}
	if len(parts) == 0 {
		return "0"
	}
	return "COALESCE(" + strings.Join(parts, ", ") + ", 0)"
}

func findTableInfo(tables []synology.TableInfo, name string) (synology.TableInfo, bool) {
	for _, table := range tables {
		if strings.EqualFold(table.Name, name) {
			return table, true
		}
	}
	return synology.TableInfo{}, false
}

func chooseSizeColumn(columnMap map[string]string) string {
	for _, name := range sizeColumnNames() {
		if original, ok := columnMap[name]; ok {
			return original
		}
	}
	var best string
	var bestScore int
	for _, original := range columnMap {
		if score := sizeColumnScore(original); score > bestScore {
			best = original
			bestScore = score
		}
	}
	return best
}

func sizeColumnScore(column string) int {
	normalized := synology.NormalizeName(column)
	compact := strings.ReplaceAll(normalized, "_", "")
	switch normalized {
	case "transferred_size", "transferred_bytes", "transfered_size", "transfered_bytes":
		return 120
	case "total_transferred_size", "total_transferred_bytes", "total_transfer_size", "total_transfer_bytes":
		return 115
	case "transfer_size", "transfer_bytes":
		return 110
	case "backup_size", "backup_bytes":
		return 90
	case "data_size", "data_bytes", "changed_size", "changed_bytes", "processed_size", "processed_bytes":
		return 80
	case "bytes", "byte_size", "total_size":
		return 60
	case "size":
		return 30
	}
	switch compact {
	case "transferredsize", "transferredbytes", "transferedsize", "transferedbytes":
		return 118
	case "totaltransferredsize", "totaltransferredbytes":
		return 113
	case "backupsize", "backupbytes":
		return 88
	case "datasize", "databytes", "processedsize", "processedbytes":
		return 78
	}
	if strings.Contains(normalized, "byte") {
		return 50
	}
	if strings.Contains(normalized, "size") &&
		!strings.Contains(normalized, "chunk_size") &&
		!strings.Contains(normalized, "block_size") &&
		!strings.Contains(normalized, "page_size") {
		return 35
	}
	return 0
}

func sizeColumnNames() []string {
	return []string{
		"transferred_size",
		"transferred_bytes",
		"transfered_size",
		"transfered_bytes",
		"transfer_size",
		"transfer_bytes",
		"total_transferred_size",
		"total_transferred_bytes",
		"total_transfer_size",
		"total_transfer_bytes",
		"backup_size",
		"backup_bytes",
		"data_size",
		"data_bytes",
		"changed_size",
		"changed_bytes",
		"processed_size",
		"processed_bytes",
		"bytes",
		"byte_size",
		"size",
		"total_size",
	}
}

func sizeJSONKeys() []string {
	return []string{
		"transferred_size",
		"transferredSize",
		"transferred_bytes",
		"transferredBytes",
		"transfered_size",
		"transferedSize",
		"transfered_bytes",
		"transferedBytes",
		"transfer_size",
		"transferSize",
		"transfer_bytes",
		"transferBytes",
		"total_transferred_size",
		"totalTransferredSize",
		"total_transferred_bytes",
		"totalTransferredBytes",
		"backup_size",
		"backupSize",
		"backup_bytes",
		"backupBytes",
		"data_size",
		"dataSize",
		"data_bytes",
		"dataBytes",
		"changed_size",
		"changedSize",
		"changed_bytes",
		"changedBytes",
		"processed_size",
		"processedSize",
		"processed_bytes",
		"processedBytes",
		"bytes",
		"size",
	}
}

func latestStructuredRun(runs []structuredRun) structuredRun {
	sort.SliceStable(runs, func(i, j int) bool {
		return structuredRunTime(runs[i]).After(structuredRunTime(runs[j]))
	})
	if len(runs) == 0 {
		return structuredRun{}
	}
	return runs[0]
}

func latestSuccessfulStructuredRun(runs []structuredRun) structuredRun {
	var matches []structuredRun
	for _, run := range runs {
		if collector.StatusFromRaw(collector.ProductABB, run.Status) == collector.StatusOK {
			matches = append(matches, run)
		}
	}
	return latestStructuredRun(matches)
}

func structuredRunTime(run structuredRun) time.Time {
	if run.End != nil {
		return *run.End
	}
	if run.Start != nil {
		return *run.Start
	}
	return time.Unix(0, 0)
}

func attachReadOnly(ctx context.Context, db *sql.DB, alias string, path string) error {
	var lastErr error
	for _, immutable := range []bool{false, true} {
		uri := synology.SQLiteURI(path, true, immutable)
		if _, err := db.ExecContext(ctx, "ATTACH DATABASE "+synology.SQLString(uri)+" AS "+synology.QuoteIdent(alias)); err == nil {
			return nil
		} else {
			lastErr = err
		}
	}
	return lastErr
}

func readRunningDeviceIDs(ctx context.Context, path string) map[string]bool {
	out := map[string]bool{}
	db, err := synology.OpenSQLiteReadOnly(path)
	if err != nil {
		return out
	}
	defer db.Close()
	rows, err := db.QueryContext(ctx, "SELECT device_id FROM running_task_table")
	if err != nil {
		return out
	}
	defer rows.Close()
	for rows.Next() {
		var value any
		if err := rows.Scan(&value); err != nil {
			continue
		}
		id := synology.StringValue(value)
		if id != "" {
			out[id] = true
		}
	}
	return out
}

func structuredJobName(run structuredRun) string {
	if run.HostName != "" {
		return strings.Split(run.HostName, ".")[0]
	}
	if run.TaskName != "" {
		return run.TaskName
	}
	if run.TaskID != "" {
		return "ABB task " + run.TaskID
	}
	return "ABB device " + run.DeviceID
}

func structuredJobID(groupID string, run structuredRun) string {
	if run.DeviceID != "" {
		return run.DeviceID
	}
	if run.TaskID != "" {
		return "task-" + run.TaskID
	}
	return groupID
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
		SizeCol:     choose(columnMap, sizeColumnNames()...),
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
