package hyperbackup

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/pthoelken/synology-activebackup-zabbixmonitor/internal/apiserver"
	"github.com/pthoelken/synology-activebackup-zabbixmonitor/internal/collector"
	"github.com/pthoelken/synology-activebackup-zabbixmonitor/internal/config"
	"github.com/pthoelken/synology-activebackup-zabbixmonitor/internal/zabbix"
)

func TestStatusSemantics(t *testing.T) {
	for _, tt := range []struct {
		name, result, activity, state string
		want                          int
	}{
		{"success", "done", "none", "backupable", collector.StatusOK},
		{"failed", "failed", "none", "backupable", collector.StatusFailed},
		{"partial", "partial", "none", "backupable", collector.StatusWarning},
		{"cancel", "cancel", "none", "backupable", collector.StatusWarning},
		{"suspend", "suspend", "none", "backupable", collector.StatusWarning},
		{"checksum", "cksum_failed", "none", "backupable", collector.StatusFailed},
		{"destination", "dest_missing", "none", "backupable", collector.StatusFailed},
		{"running overrides old failure", "failed", "backingup", "backupable", collector.StatusRunning},
		{"current failure overrides success", "done", "dest_missing", "backupable", collector.StatusFailed},
		{"repository failure overrides success", "done", "none", "broken", collector.StatusFailed},
		{"unauthorized", "done", "none", "unauth", collector.StatusFailed},
		{"unknown repository", "done", "none", "new_state", collector.StatusUnknown},
		{"relinkable", "done", "none", "relinkable", collector.StatusWarning},
		{"unknown", "new_status", "none", "backupable", collector.StatusUnknown},
		{"unknown activity", "done", "new_activity", "backupable", collector.StatusUnknown},
		{"numeric codes are not ABB codes", "2", "none", "backupable", collector.StatusUnknown},
		{"never run", "none", "none", "backupable", collector.StatusNoData},
	} {
		t.Run(tt.name, func(t *testing.T) {
			j := collector.Job{}
			s := status{Result: &tt.result, Activity: tt.activity, State: tt.state, Start: json.RawMessage(`1700000000`), End: json.RawMessage(`1700000060`)}
			if err := fillJob(&j, s, time.Unix(1700000120, 0)); err != nil {
				t.Fatal(err)
			}
			if j.Status != tt.want || j.RuntimeSeconds != 60 || j.AgeSeconds != 60 {
				t.Fatalf("unexpected job: %+v", j)
			}
			if tt.result != "done" && j.LastSuccessTime != nil {
				t.Fatal("fabricated successful backup")
			}
		})
	}
}

func TestTimeSemantics(t *testing.T) {
	for _, raw := range []string{`1700000000`, `"1700000000"`, `"2026/09/14 12:30"`, `"2026/09/14 12:30:00"`, `"2026-09-14T12:30:00+02:00"`} {
		if got, err := parseTime(json.RawMessage(raw)); err != nil || got == nil {
			t.Fatalf("%s: %v, %v", raw, got, err)
		}
	}
	for _, raw := range []string{`null`, `""`, `0`, `"0"`, ``} {
		if got, err := parseTime(json.RawMessage(raw)); err != nil || got != nil {
			t.Fatalf("missing %s: %v, %v", raw, got, err)
		}
	}
	for _, raw := range []string{`"tomorrow"`, `{}`, `true`, `-1`} {
		if _, err := parseTime(json.RawMessage(raw)); err == nil {
			t.Fatalf("accepted %s", raw)
		}
	}
	done := "done"
	j := collector.Job{}
	if err := fillJob(&j, status{Result: &done}, time.Now()); err != nil {
		t.Fatal(err)
	}
	if j.Status != collector.StatusNoData || j.LastSuccessTime != nil {
		t.Fatalf("fabricated success: %+v", j)
	}
	failed := "failed"
	if err := fillJob(&j, status{Result: &failed, End: json.RawMessage(`1700000060`), Success: json.RawMessage(`1699990000`)}, time.Unix(1700000120, 0)); err != nil {
		t.Fatal(err)
	}
	if j.LastSuccessAgeSeconds != 10120 {
		t.Fatalf("lost previous success: %+v", j)
	}
}

func TestCollectionAndZabbix(t *testing.T) {
	calls := 0
	c := Collector{run: func(ctx context.Context, args ...string) ([]byte, error) {
		if _, ok := ctx.Deadline(); !ok {
			t.Fatal("missing deadline")
		}
		calls++
		prefix := []string{"--exec", "api=SYNO.Backup.Task", "version=1"}
		if !reflect.DeepEqual(args[:3], prefix) {
			t.Fatalf("unexpected command %v", args)
		}
		if calls == 1 {
			if !reflect.DeepEqual(args[3:], []string{"method=list"}) {
				t.Fatal(args)
			}
			return []byte(`{"success":true,"data":{"task_list":[{"task_id":1,"name":"NAS & USB","target_type":"local"},{"task_id":"2","name":"New"},{"task_id":3,"name":"Unavailable"}]}}`), nil
		}
		if args[3] != "method=status" || args[5] != "blOnline=false" || len(args) != 7 {
			t.Fatal(args)
		}
		switch args[4] {
		case "task_id=1":
			return []byte(`{"success":true,"data":{"state":"backupable","status":"none","last_bkp_result":"done","last_bkp_time":1700000000,"last_bkp_end_time":1700000060}}`), nil
		case "task_id=2":
			return []byte(`{"success":true,"data":{"status":"none","last_bkp_result":"none"}}`), nil
		default:
			return []byte(`{"success":false,"error":{"code":105}}`), nil
		}
	}}
	now := time.Unix(1700000120, 0)
	result := c.Collect(context.Background(), now)
	if len(result.Jobs) != 3 || len(result.Errors) != 1 || result.Sources[0].Error == "" {
		t.Fatalf("%+v", result)
	}
	if result.Jobs[2].Status != collector.StatusUnknown || !result.Jobs[2].HasData {
		t.Fatal("unavailable task disappeared")
	}
	snapshot := collector.Snapshot{CollectedAt: now, Jobs: result.Jobs, Health: collector.Health{JobCount: 3, DBMissing: []string{collector.ProductHyperBackup}}}
	cfg := config.Default()
	cfg.API.Token = "test-token"
	cfg.Zabbix.Sender.Host = "test-nas"
	values, err := zabbix.SnapshotSenderValues(cfg, snapshot)
	if err != nil {
		t.Fatal(err)
	}
	fields := map[string]string{}
	for _, v := range values {
		fields[v.Key] = v.Value
	}
	for key, want := range map[string]string{
		"synology.activebackup.job.status[hyperbackup,1]":           "1",
		"synology.activebackup.job.status[hyperbackup,2]":           "8",
		"synology.activebackup.job.status[hyperbackup,3]":           "10",
		"synology.activebackup.product.db_missing[hyperbackup]":     "1",
		"synology.activebackup.job.last_success_age[hyperbackup,1]": "60",
	} {
		if fields[key] != want {
			t.Fatalf("%s = %s, want %s", key, fields[key], want)
		}
	}
	var discovery zabbix.Discovery
	if err := json.Unmarshal([]byte(fields["synology.activebackup.discovery"]), &discovery); err != nil || len(discovery.Data) != 3 {
		t.Fatalf("discovery: %+v %v", discovery, err)
	}
	store := collector.NewStore()
	store.Set(snapshot)
	handler := apiserver.New(cfg, "", store, nil).Handler()
	for path, want := range map[string]string{
		"/api/v1/job?product=hyperbackup&task_id=1&field=status": "1",
		"/api/v1/job?product=hyperbackup&task_id=2&field=status": "8",
		"/api/v1/health?product=hyperbackup&field=db_missing":    "1",
	} {
		req := httptest.NewRequest("GET", path, nil)
		req.Header.Set("Authorization", "Bearer test-token")
		res := httptest.NewRecorder()
		handler.ServeHTTP(res, req)
		if res.Code != 200 || strings.TrimSpace(res.Body.String()) != want {
			t.Fatalf("%s: %d %s", path, res.Code, res.Body.String())
		}
	}
}

func TestBadResponses(t *testing.T) {
	for _, raw := range []string{`not-json`, `{"success":false,"error":{"code":105}}`, `{"success":true}`, `{"success":true,"data":null}`, `{"success":true,"data":{}}`, `{"success":true,"data":{"task_list":null}}`, `{"success":true,"data":{"task_list":[{"task_id":"1;touch /tmp/no"}]}}`, `{"success":true,"data":{"task_list":[{"task_id":1},{"task_id":1}]}}`} {
		c := Collector{run: func(context.Context, ...string) ([]byte, error) { return []byte(raw), nil }}
		r := c.Collect(context.Background(), time.Now())
		if len(r.Errors) == 0 || r.Sources[0].Error == "" {
			t.Fatalf("accepted: %s", raw)
		}
	}
	for _, raw := range []string{`{"success":true,"data":{"task_list":[]}}`} {
		r := (Collector{run: func(context.Context, ...string) ([]byte, error) { return []byte(raw), nil }}).Collect(context.Background(), time.Now())
		if len(r.Errors) != 0 || !r.Sources[0].Found || len(r.Jobs) != 0 {
			t.Fatalf("empty list: %+v", r)
		}
	}
	c := Collector{run: func(context.Context, ...string) ([]byte, error) { return nil, errors.New("permission denied") }}
	if r := c.Collect(context.Background(), time.Now()); len(r.Errors) != 1 || r.Sources[0].Found {
		t.Fatal(r)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	c.run = func(context.Context, ...string) ([]byte, error) {
		t.Fatal("executed after cancellation")
		return nil, nil
	}
	if r := c.Collect(ctx, time.Now()); len(r.Errors) != 1 {
		t.Fatal(r)
	}
}

func TestMalformedStatus(t *testing.T) {
	for _, raw := range []string{`{}`, `{"last_bkp_result":null}`, `{"last_bkp_result":"done","last_bkp_end_time":"invalid"}`} {
		var s status
		if err := json.Unmarshal([]byte(raw), &s); err != nil {
			t.Fatal(err)
		}
		if err := fillJob(&collector.Job{}, s, time.Now()); err == nil {
			t.Fatalf("accepted %s", raw)
		}
	}
}

func TestOutputLimit(t *testing.T) {
	var copied boundedBuffer
	if _, err := io.Copy(&copied, io.LimitReader(strings.NewReader(strings.Repeat("x", maxOutput+1)), maxOutput+1)); err == nil || copied.Len() > maxOutput {
		t.Fatal("streaming copy bypassed output limit")
	}
	var b boundedBuffer
	if _, err := b.Write(make([]byte, maxOutput)); err != nil {
		t.Fatal(err)
	}
	if _, err := b.Write([]byte("x")); err == nil || b.Len() != maxOutput {
		t.Fatal("output limit not enforced")
	}
}
