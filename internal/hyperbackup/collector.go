// Package hyperbackup reads Hyper Backup task metadata through DSM's local API.
package hyperbackup

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/pthoelken/synology-activebackup-zabbixmonitor/internal/collector"
	"github.com/pthoelken/synology-activebackup-zabbixmonitor/internal/synology"
)

const webAPI = "/usr/syno/bin/synowebapi"
const maxOutput = 4 << 20

// Collector uses only list/status, never backup or configuration operations.
// run is injectable for tests; the executable is deliberately not configurable.
type Collector struct {
	API APIConfig
	run func(context.Context, ...string) ([]byte, error)
}

type task struct {
	ID         json.RawMessage `json:"task_id"`
	Name       string          `json:"name"`
	TargetType string          `json:"target_type"`
}

type status struct {
	State    string          `json:"state"`
	Activity string          `json:"status"`
	Result   *string         `json:"last_bkp_result"`
	Start    json.RawMessage `json:"last_bkp_time"`
	End      json.RawMessage `json:"last_bkp_end_time"`
	Success  json.RawMessage `json:"last_bkp_success_time"`
}

var taskIDPattern = regexp.MustCompile(`^[0-9]+$`)

func (c Collector) Collect(ctx context.Context, now time.Time) collector.Result {
	// Bound the whole poll as well as individual calls, including large task lists.
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	source := collector.Source{Product: collector.ProductHyperBackup, Path: webAPI, Kind: "SYNO.Backup.Task", Found: true}
	result := collector.Result{}
	recordError := func(err error) {
		source.Error = "Hyper Backup query failed; see collector_errors"
		result.Errors = append(result.Errors, fmt.Errorf("hyperbackup: %w", err))
	}

	if c.API.URL != "" {
		source.Path = "DSM HTTPS API"
		client, err := newAPIClient(c.API)
		if err == nil {
			defer client.close()
			err = client.login(ctx, c.API)
		}
		if err != nil {
			source.Found = false
			recordError(err)
			result.Sources = []collector.Source{source}
			return result
		}
		c.run = client.run
	}
	var listing struct {
		Tasks *[]task `json:"task_list"`
	}
	if err := c.call(ctx, &listing, "method=list"); err != nil {
		source.Found = false
		recordError(err)
	} else if listing.Tasks == nil {
		recordError(errors.New("list response has no task_list array"))
	} else {
		seen := map[string]bool{}
		for _, t := range *listing.Tasks {
			id := strings.Trim(string(t.ID), `"`)
			if !taskIDPattern.MatchString(id) || seen[id] {
				recordError(errors.New("list response contains an invalid or duplicate task ID"))
				continue
			}
			seen[id] = true
			name := t.Name
			if name == "" {
				name = "Hyper Backup task " + id
			}
			job := collector.Job{Product: collector.ProductHyperBackup, TaskID: id, JobName: name, BackupType: t.TargetType, Status: collector.StatusUnknown, HasData: true}
			var s status
			err := c.call(ctx, &s, "method=status", "task_id="+id, "blOnline=false", `additional=["last_bkp_time","next_bkp_time","last_bkp_result","is_modified","last_bkp_progress"]`)
			if err == nil {
				err = fillJob(&job, s, now)
			}
			if err != nil {
				job.Status = collector.StatusUnknown
				job.ErrorCode = "collector_error"
				recordError(fmt.Errorf("task %s: %w", id, err))
			}
			result.Jobs = append(result.Jobs, job)
		}
	}
	sort.Slice(result.Jobs, func(i, j int) bool { return result.Jobs[i].TaskID < result.Jobs[j].TaskID })
	result.Sources = []collector.Source{source}
	return result
}

func (c Collector) call(ctx context.Context, target any, args ...string) error {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	if err := ctx.Err(); err != nil {
		return err
	}
	run := c.run
	if run == nil {
		run = runCommand
	}
	output, err := run(ctx, append([]string{"--exec", "api=SYNO.Backup.Task", "version=1"}, args...)...)
	if err != nil {
		return fmt.Errorf("DSM API unavailable (check Hyper Backup installation and access permissions): %w", err)
	}
	var response struct {
		Success bool            `json:"success"`
		Data    json.RawMessage `json:"data"`
		Error   struct {
			Code int `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(output, &response); err != nil {
		return errors.New("invalid JSON from DSM API")
	}
	if !response.Success {
		return fmt.Errorf("DSM API error %d (check Hyper Backup availability and access permissions)", response.Error.Code)
	}
	if len(response.Data) == 0 || bytes.Equal(bytes.TrimSpace(response.Data), []byte("null")) {
		return errors.New("DSM API returned no data")
	}
	if err := json.Unmarshal(response.Data, target); err != nil {
		return errors.New("unexpected DSM API data schema")
	}
	return nil
}

// Limit output in memory and keep stderr separate: DSM diagnostic messages must
// neither corrupt JSON nor expose task configuration in logs.
type boundedBuffer struct{ buffer bytes.Buffer }

func (b *boundedBuffer) Len() int      { return b.buffer.Len() }
func (b *boundedBuffer) Bytes() []byte { return b.buffer.Bytes() }

func (b *boundedBuffer) Write(p []byte) (int, error) {
	if b.Len()+len(p) > maxOutput {
		return 0, errors.New("DSM API output exceeds 4 MiB")
	}
	return b.buffer.Write(p)
}
func runCommand(ctx context.Context, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, webAPI, args...)
	var output boundedBuffer
	cmd.Stdout = &output
	cmd.Stderr = io.Discard
	cmd.WaitDelay = time.Second
	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, err
	}
	return output.Bytes(), nil
}

func fillJob(job *collector.Job, s status, now time.Time) error {
	if s.Result == nil {
		return errors.New("status response has no last_bkp_result")
	}
	job.RawStatus = strings.ToLower(strings.TrimSpace(*s.Result))
	job.Status = collector.StatusFromRaw(collector.ProductHyperBackup, job.RawStatus)
	activity := strings.ToLower(strings.TrimSpace(s.Activity))
	state := strings.ToLower(strings.TrimSpace(s.State))
	// Current activity is separate from the outcome of the previous backup.
	if activity != "" && activity != "none" {
		job.RawStatus = activity
		job.Status = collector.StatusFromRaw(collector.ProductHyperBackup, activity)
	}
	switch state {
	case "", "backupable", "error_detect":
	case "exportable", "importable", "relinkable":
		job.Status = collector.StatusWarning
	case "broken", "unauth", "endofservice", "restore_only":
		job.Status = collector.StatusFailed
		job.ErrorCode = state
	default:
		job.Status = collector.StatusUnknown
	}
	job.Info = map[string]string{"state": s.State, "activity": s.Activity, "last_bkp_result": *s.Result}
	var err error
	if job.StartTime, err = parseTime(s.Start); err != nil {
		return fmt.Errorf("last_bkp_time: %w", err)
	}
	if job.EndTime, err = parseTime(s.End); err != nil {
		return fmt.Errorf("last_bkp_end_time: %w", err)
	}
	if job.LastSuccessTime, err = parseTime(s.Success); err != nil {
		return fmt.Errorf("last_bkp_success_time: %w", err)
	}
	// Only an explicitly successful backup may supply a missing success timestamp.
	if job.LastSuccessTime == nil && collector.StatusFromRaw(collector.ProductHyperBackup, *s.Result) == collector.StatusOK {
		job.LastSuccessTime = job.EndTime
	}
	if job.Status == collector.StatusOK && job.EndTime == nil {
		job.Status = collector.StatusNoData
	}
	job.RuntimeSeconds = synology.RuntimeSeconds(job.StartTime, job.EndTime)
	if job.EndTime != nil {
		job.LastEndUnix = job.EndTime.Unix()
		job.AgeSeconds = synology.AgeSeconds(now, job.EndTime)
	}
	job.LastSuccessAgeSeconds = synology.AgeSeconds(now, job.LastSuccessTime)
	job.Info["status"] = collector.StatusName(job.Status)
	// The status endpoint does not document transferred bytes. Leave size at zero.
	return nil
}

func parseTime(raw json.RawMessage) (*time.Time, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return nil, nil
	}
	var value any
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if err := decoder.Decode(&value); err != nil {
		return nil, errors.New("invalid timestamp")
	}
	s := strings.TrimSpace(synology.StringValue(value))
	if s == "" || s == "0" {
		return nil, nil
	}
	if t := synology.ParseTimeValue(s); t != nil {
		return t, nil
	}
	// Hyper Backup also returns NAS-local timestamps without seconds.
	if t, err := time.ParseInLocation("2006/01/02 15:04", s, time.Local); err == nil {
		return &t, nil
	}
	return nil, errors.New("unsupported timestamp format")
}
