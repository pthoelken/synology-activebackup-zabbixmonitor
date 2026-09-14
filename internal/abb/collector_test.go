package abb

import (
	"context"
	"database/sql"
	"strings"
	"testing"
)

func TestStructuredABBDeviceQueryMatchesResultToDevice(t *testing.T) {
	query := structuredABBDeviceQuery(true, "0", false)
	want := "AND drt.config_device_id = dt.device_id"
	if !strings.Contains(query, want) {
		t.Fatalf("structuredABBDeviceQuery() must contain %q", want)
	}
}

func TestStructuredABBDeviceQueryKeepsPerDeviceStatus(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	statements := []string{
		`ATTACH DATABASE ':memory:' AS activity`,
		`CREATE TABLE device_table (host_name TEXT, device_id INTEGER)`,
		`CREATE TABLE backup_task_device (device_id INTEGER, task_id INTEGER)`,
		`CREATE TABLE task_table (task_id INTEGER, task_name TEXT, source_type INTEGER, backup_type INTEGER, sched_content TEXT)`,
		`CREATE TABLE activity.result_table (result_id INTEGER, task_id INTEGER, task_config TEXT, time_start INTEGER, time_end INTEGER, job_action INTEGER)`,
		`CREATE TABLE activity.device_result_table (result_id INTEGER, config_device_id INTEGER, status INTEGER)`,
		`INSERT INTO device_table VALUES ('vm-ok', 12), ('vm-warning', 14)`,
		`INSERT INTO backup_task_device VALUES (12, 1), (14, 1)`,
		`INSERT INTO task_table VALUES (1, 'Backup X1', 0, 1, '{"schedule_setting_type":1}')`,
		`INSERT INTO activity.result_table VALUES (100, 1, '', 1000, 1100, 1)`,
		`INSERT INTO activity.device_result_table VALUES (100, 12, 2), (100, 14, 5)`,
	}
	for _, statement := range statements {
		if _, err := db.Exec(statement); err != nil {
			t.Fatalf("execute %q: %v", statement, err)
		}
	}

	runs, err := readStructuredRuns(context.Background(), db, structuredABBDeviceQuery(true, "0", false), "activity.db")
	if err != nil {
		t.Fatal(err)
	}
	statuses := make(map[string]string, len(runs))
	for _, run := range runs {
		statuses[run.DeviceID] = run.Status
	}
	if got := statuses["12"]; got != "2" {
		t.Fatalf("device 12 status = %q, want 2", got)
	}
	if got := statuses["14"]; got != "5" {
		t.Fatalf("device 14 status = %q, want 5", got)
	}
}
