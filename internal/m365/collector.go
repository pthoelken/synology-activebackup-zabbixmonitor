package m365

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
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

type taskNameCandidate struct {
	Name   string
	Score  int
	Source string
}

type taskSizeCandidate struct {
	Size   int64
	Score  int
	Source string
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
	sizeCol := m365SizeColumn(tables)
	sizeExpr := "0"
	if sizeCol != "" {
		sizeExpr = synology.QuoteIdent(sizeCol)
	}

	query := fmt.Sprintf(`
SELECT task_id, task_execution_id, execution_status, error_code, start_run_time, end_run_time,
       job_type, service_type, selected_item, user_name, to_user_name, %s
FROM job_log_table`, sizeExpr)
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

	taskNames := c.collectTaskNames(ctx, path)
	taskSizes := collectTaskSizes(ctx, db, tables)
	var jobs []collector.Job
	for taskID, runs := range grouped {
		latest := latestRun(runs)
		success := latestSuccessfulRun(runs)
		name := c.jobName(taskID, latest)
		if taskName, ok := taskNames[taskID]; ok && taskName != "" {
			name = taskName
		}
		transferredSize := latest.TransferredSize
		transferredSizeSource := ""
		if transferredSize > 0 && sizeCol != "" {
			transferredSizeSource = "job_log_table." + sizeCol
		}
		if transferredSize == 0 {
			if candidate, ok := taskSizes[taskID]; ok && candidate.Size > 0 {
				transferredSize = candidate.Size
				transferredSizeSource = candidate.Source
			}
		}
		status := collector.StatusFromRaw(collector.ProductM365, latest.Status)
		job := collector.Job{
			Product:         collector.ProductM365,
			TaskID:          taskID,
			JobName:         name,
			ServiceType:     latest.ServiceType,
			BackupType:      latest.JobType,
			Status:          status,
			RawStatus:       latest.Status,
			ErrorCode:       latest.ErrorCode,
			StartTime:       latest.Start,
			EndTime:         latest.End,
			LastSuccessTime: success.End,
			RuntimeSeconds:  synology.RuntimeSeconds(latest.Start, latest.End),
			TransferredSize: transferredSize,
			HasData:         true,
			SourceDB:        path,
			Info: map[string]string{
				"task_execution_id": latest.TaskExecutionID,
				"status":            collector.StatusName(status),
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
		if taskName, ok := taskNames[taskID]; ok && taskName != "" {
			job.Info["task_name"] = taskName
		}
		if transferredSize > 0 && transferredSizeSource != "" {
			job.Info["transferred_size_source"] = transferredSizeSource
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

func (c Collector) collectTaskNames(ctx context.Context, logDBPath string) map[string]string {
	best := map[string]taskNameCandidate{}
	for _, path := range m365TaskNameDBCandidates(logDBPath) {
		db, err := synology.OpenSQLiteReadOnly(path)
		if err != nil {
			continue
		}
		c.collectTaskNamesFromDB(ctx, db, path, best)
		_ = db.Close()
	}

	names := map[string]string{}
	for taskID, candidate := range best {
		names[taskID] = candidate.Name
	}
	return names
}

func m365TaskNameDBCandidates(logDBPath string) []string {
	dir := filepath.Dir(logDBPath)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return []string{logDBPath}
	}

	seen := map[string]struct{}{}
	var paths []string
	add := func(path string) {
		clean := filepath.Clean(path)
		if _, ok := seen[clean]; ok {
			return
		}
		seen[clean] = struct{}{}
		paths = append(paths, clean)
	}
	add(logDBPath)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := strings.ToLower(entry.Name())
		if strings.HasSuffix(name, ".sqlite") || strings.HasSuffix(name, ".sqlite3") || strings.HasSuffix(name, ".db") {
			add(filepath.Join(dir, entry.Name()))
		}
	}
	sort.Strings(paths)
	return paths
}

func (c Collector) collectTaskNamesFromDB(ctx context.Context, db *sql.DB, dbPath string, best map[string]taskNameCandidate) {
	tables, err := synology.ListTables(db)
	if err != nil {
		return
	}

	for _, table := range tables {
		columns := synology.ColumnNames(table.Columns)
		idCol := chooseM365TaskIDColumn(columns)
		if idCol == "" {
			continue
		}
		nameCols := chooseM365TaskNameColumns(table.Columns)
		if len(nameCols) == 0 {
			continue
		}
		query := "SELECT " + synology.QuoteIdent(idCol)
		for _, col := range nameCols {
			query += ", " + synology.QuoteIdent(col)
		}
		query += " FROM " + synology.QuoteIdent(table.Name) + " LIMIT 2000"

		rows, err := db.QueryContext(ctx, query)
		if err != nil {
			continue
		}
		for rows.Next() {
			values := make([]any, len(nameCols)+1)
			dest := make([]any, len(values))
			for i := range values {
				dest[i] = &values[i]
			}
			if err := rows.Scan(dest...); err != nil {
				continue
			}
			taskID := strings.TrimSpace(synology.StringValue(values[0]))
			if taskID == "" {
				continue
			}
			for i, col := range nameCols {
				raw := synology.StringValue(values[i+1])
				name, score := m365TaskNameFromValue(col, raw)
				if name == "" {
					continue
				}
				sourceScore := score
				if strings.Contains(strings.ToLower(table.Name), "task") {
					sourceScore += 10
				}
				candidate := taskNameCandidate{Name: name, Score: sourceScore, Source: dbPath + ":" + table.Name + "." + col}
				if current, ok := best[taskID]; !ok || candidate.Score > current.Score || (candidate.Score == current.Score && len(candidate.Name) > len(current.Name)) {
					best[taskID] = candidate
				}
			}
		}
		_ = rows.Close()
	}
}

func chooseM365TaskIDColumn(columns map[string]string) string {
	for _, name := range []string{
		"task_id",
		"taskid",
		"task_uuid",
		"task_guid",
		"task_no",
		"job_id",
		"jobid",
		"id",
	} {
		if col, ok := columns[name]; ok {
			return col
		}
	}
	return ""
}

func chooseM365TaskNameColumns(columns []synology.ColumnInfo) []string {
	preferred := map[string]int{
		"task_name":        120,
		"taskname":         120,
		"backup_task_name": 120,
		"display_name":     115,
		"displayname":      115,
		"name":             110,
		"title":            105,
		"subject":          90,
		"tenant_name":      80,
		"account_name":     80,
		"organization":     80,
		"org_name":         80,
	}
	var out []string
	seen := map[string]struct{}{}
	add := func(name string) {
		if _, ok := seen[name]; ok {
			return
		}
		seen[name] = struct{}{}
		out = append(out, name)
	}
	for _, col := range columns {
		if _, ok := preferred[synology.NormalizeName(col.Name)]; ok {
			add(col.Name)
		}
	}
	for _, col := range columns {
		normalized := synology.NormalizeName(col.Name)
		typ := strings.ToLower(col.Type)
		if strings.Contains(normalized, "config") ||
			strings.Contains(normalized, "setting") ||
			strings.Contains(normalized, "info") ||
			strings.Contains(normalized, "detail") ||
			strings.Contains(normalized, "meta") ||
			strings.Contains(typ, "text") ||
			strings.Contains(typ, "char") ||
			strings.Contains(typ, "clob") {
			add(col.Name)
		}
	}
	return out
}

func m365TaskNameFromValue(column string, raw string) (string, int) {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "0" || raw == "-1" {
		return "", 0
	}
	if name, score := extractM365TaskNameFromJSON(raw); name != "" {
		return name, score
	}
	if !looksLikeM365TaskName(raw) {
		return "", 0
	}

	normalized := synology.NormalizeName(column)
	score := 60
	switch normalized {
	case "task_name", "taskname", "backup_task_name":
		score = 120
	case "display_name", "displayname":
		score = 115
	case "name":
		score = 110
	case "title":
		score = 105
	}
	if strings.Contains(strings.ToLower(raw), "m365 backup") {
		score += 20
	}
	return raw, score
}

func extractM365TaskNameFromJSON(raw string) (string, int) {
	first := strings.TrimSpace(raw)
	if !strings.HasPrefix(first, "{") && !strings.HasPrefix(first, "[") {
		return "", 0
	}
	var value any
	if err := json.Unmarshal([]byte(first), &value); err != nil {
		return "", 0
	}
	return findM365TaskNameInJSON(value)
}

func findM365TaskNameInJSON(value any) (string, int) {
	switch typed := value.(type) {
	case map[string]any:
		for _, key := range []string{"task_name", "taskName", "backup_task_name", "display_name", "displayName", "name", "title"} {
			if raw, ok := typed[key]; ok {
				name := strings.TrimSpace(synology.StringValue(raw))
				if looksLikeM365TaskName(name) {
					score := 100
					if strings.Contains(strings.ToLower(key), "task") {
						score += 20
					}
					return name, score
				}
			}
		}
		var bestName string
		var bestScore int
		for _, raw := range typed {
			name, score := findM365TaskNameInJSON(raw)
			if score > bestScore || (score == bestScore && len(name) > len(bestName)) {
				bestName = name
				bestScore = score
			}
		}
		return bestName, bestScore
	case []any:
		var bestName string
		var bestScore int
		for _, raw := range typed {
			name, score := findM365TaskNameInJSON(raw)
			if score > bestScore || (score == bestScore && len(name) > len(bestName)) {
				bestName = name
				bestScore = score
			}
		}
		return bestName, bestScore
	case string:
		if looksLikeM365TaskName(typed) {
			return strings.TrimSpace(typed), 60
		}
	}
	return "", 0
}

func looksLikeM365TaskName(value string) bool {
	value = strings.TrimSpace(value)
	if len(value) < 3 || len(value) > 180 {
		return false
	}
	lower := strings.ToLower(value)
	if strings.HasPrefix(lower, "/volume") ||
		strings.Contains(lower, ".sqlite") ||
		strings.Contains(lower, "@activebackup") ||
		strings.HasPrefix(lower, "{") ||
		strings.HasPrefix(lower, "[") {
		return false
	}
	if _, err := time.Parse(time.RFC3339, value); err == nil {
		return false
	}
	if _, err := fmt.Sscan(value, new(int64)); err == nil && strings.Trim(value, "0123456789") == "" {
		return false
	}
	return true
}

func m365SizeColumn(tables []synology.TableInfo) string {
	for _, table := range tables {
		if !strings.EqualFold(table.Name, "job_log_table") {
			continue
		}
		columns := synology.ColumnNames(table.Columns)
		var best string
		var bestScore int
		for _, col := range table.Columns {
			if original, ok := columns[synology.NormalizeName(col.Name)]; ok {
				score := sizeColumnScore(original)
				if score > bestScore {
					best = original
					bestScore = score
				}
			}
		}
		if best != "" {
			return best
		}
	}
	return ""
}

func collectTaskSizes(ctx context.Context, db *sql.DB, tables []synology.TableInfo) map[string]taskSizeCandidate {
	out := map[string]taskSizeCandidate{}
	for _, table := range tables {
		columns := synology.ColumnNames(table.Columns)
		idCol := chooseM365TaskIDColumn(columns)
		if idCol == "" {
			continue
		}
		sizeCols := chooseM365SizeColumns(table.Columns, idCol)
		jsonCols := chooseM365JSONSizeColumns(table.Columns, idCol)
		if len(sizeCols) == 0 && len(jsonCols) == 0 {
			continue
		}

		selectCols := append([]string{idCol}, sizeCols...)
		selectCols = append(selectCols, jsonCols...)
		query := "SELECT " + quoteM365Columns(selectCols) + " FROM " + synology.QuoteIdent(table.Name) + " LIMIT 10000"
		rows, err := db.QueryContext(ctx, query)
		if err != nil {
			continue
		}
		for rows.Next() {
			values := make([]any, len(selectCols))
			dest := make([]any, len(values))
			for i := range values {
				dest[i] = &values[i]
			}
			if err := rows.Scan(dest...); err != nil {
				continue
			}
			taskID := strings.TrimSpace(synology.StringValue(values[0]))
			if taskID == "" {
				continue
			}
			for i, col := range sizeCols {
				size := synology.Int64Value(values[i+1])
				score := sizeColumnScore(col)
				recordTaskSize(out, taskID, taskSizeCandidate{
					Size:   size,
					Score:  score,
					Source: table.Name + "." + col,
				})
			}
			jsonOffset := 1 + len(sizeCols)
			for i, col := range jsonCols {
				size, score := extractM365SizeFromJSON(synology.StringValue(values[jsonOffset+i]))
				if score == 0 {
					continue
				}
				recordTaskSize(out, taskID, taskSizeCandidate{
					Size:   size,
					Score:  score,
					Source: table.Name + "." + col,
				})
			}
		}
		_ = rows.Close()
	}
	return out
}

func chooseM365SizeColumns(columns []synology.ColumnInfo, idCol string) []string {
	var out []string
	for _, col := range columns {
		if strings.EqualFold(col.Name, idCol) {
			continue
		}
		if sizeColumnScore(col.Name) > 0 {
			out = append(out, col.Name)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		return sizeColumnScore(out[i]) > sizeColumnScore(out[j])
	})
	if len(out) > 8 {
		out = out[:8]
	}
	return out
}

func chooseM365JSONSizeColumns(columns []synology.ColumnInfo, idCol string) []string {
	var out []string
	for _, col := range columns {
		if strings.EqualFold(col.Name, idCol) || sizeColumnScore(col.Name) > 0 {
			continue
		}
		normalized := synology.NormalizeName(col.Name)
		typ := strings.ToLower(col.Type)
		if strings.Contains(normalized, "param") ||
			strings.Contains(normalized, "config") ||
			strings.Contains(normalized, "info") ||
			strings.Contains(normalized, "detail") ||
			strings.Contains(normalized, "stat") ||
			strings.Contains(typ, "text") ||
			strings.Contains(typ, "char") ||
			strings.Contains(typ, "clob") {
			out = append(out, col.Name)
		}
	}
	if len(out) > 8 {
		out = out[:8]
	}
	return out
}

func recordTaskSize(out map[string]taskSizeCandidate, taskID string, candidate taskSizeCandidate) {
	if candidate.Size <= 0 || candidate.Score <= 0 {
		return
	}
	current, ok := out[taskID]
	if !ok ||
		candidate.Score > current.Score ||
		(candidate.Score == current.Score && candidate.Size > current.Size) {
		out[taskID] = candidate
	}
}

func quoteM365Columns(columns []string) string {
	quoted := make([]string, 0, len(columns))
	for _, col := range columns {
		quoted = append(quoted, synology.QuoteIdent(col))
	}
	return strings.Join(quoted, ", ")
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
	case "totaltransferredsize", "totaltransferredbytes", "totaltransfersize", "totaltransferbytes":
		return 113
	case "transfersize", "transferbytes":
		return 108
	case "backupsize", "backupbytes":
		return 88
	case "datasize", "databytes", "changedsize", "changedbytes", "processedsize", "processedbytes":
		return 78
	case "bytes", "bytesize", "totalsize":
		return 58
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

func extractM365SizeFromJSON(raw string) (int64, int) {
	raw = strings.TrimSpace(raw)
	if raw == "" || (!strings.HasPrefix(raw, "{") && !strings.HasPrefix(raw, "[")) {
		return 0, 0
	}
	var value any
	if err := json.Unmarshal([]byte(raw), &value); err != nil {
		return 0, 0
	}
	return findM365SizeInJSON(value)
}

func findM365SizeInJSON(value any) (int64, int) {
	switch typed := value.(type) {
	case map[string]any:
		var bestSize int64
		var bestScore int
		for key, raw := range typed {
			score := sizeColumnScore(key)
			if score > 0 {
				size := synology.Int64Value(raw)
				if size > 0 && (score > bestScore || (score == bestScore && size > bestSize)) {
					bestSize = size
					bestScore = score
				}
			}
			nestedSize, nestedScore := findM365SizeInJSON(raw)
			if nestedScore > 0 {
				nestedScore -= 5
			}
			if nestedSize > 0 && (nestedScore > bestScore || (nestedScore == bestScore && nestedSize > bestSize)) {
				bestSize = nestedSize
				bestScore = nestedScore
			}
		}
		return bestSize, bestScore
	case []any:
		var bestSize int64
		var bestScore int
		for _, raw := range typed {
			size, score := findM365SizeInJSON(raw)
			if size > 0 && (score > bestScore || (score == bestScore && size > bestSize)) {
				bestSize = size
				bestScore = score
			}
		}
		return bestSize, bestScore
	case string:
		return 0, 0
	default:
		return 0, 0
	}
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
		if collector.StatusFromRaw(collector.ProductM365, r.Status) == collector.StatusOK {
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
