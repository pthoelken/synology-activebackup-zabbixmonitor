package zabbix

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/pthoelken/synology-activebackup-zabbixmonitor/internal/collector"
	"github.com/pthoelken/synology-activebackup-zabbixmonitor/internal/config"
	"gopkg.in/yaml.v3"
)

func TestHyperBackupTemplateContract(t *testing.T) {
	for _, path := range []string{"../../zabbix/template_synology_activebackup_zabbix_7.4.yaml", "../../zabbix/template_synology_activebackup_zabbix_sender_7.4.yaml"} {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		var root map[string]any
		if err := yaml.Unmarshal(data, &root); err != nil {
			t.Fatal(err)
		}
		ids := map[string]bool{}
		keys := map[string]bool{}
		var visit func(any)
		visit = func(v any) {
			switch x := v.(type) {
			case map[string]any:
				if id, ok := x["uuid"].(string); ok {
					if ids[id] {
						t.Fatalf("duplicate uuid in %s: %s", path, id)
					}
					ids[id] = true
				}
				if key, ok := x["key"].(string); ok {
					keys[key] = true
				}
				for _, child := range x {
					visit(child)
				}
			case []any:
				for _, child := range x {
					visit(child)
				}
			}
		}
		visit(root)
		if !keys["synology.activebackup.product.db_missing[hyperbackup]"] {
			t.Fatalf("missing Hyper Backup source item in %s", path)
		}
		if strings.Contains(path, "sender") {
			cfg := config.Default()
			cfg.Zabbix.Sender.Host = "nas"
			values, err := SnapshotSenderValues(cfg, collector.Snapshot{CollectedAt: time.Now(), Jobs: []collector.Job{{Product: collector.ProductHyperBackup, TaskID: "1", HasData: true}}})
			if err != nil {
				t.Fatal(err)
			}
			for _, v := range values {
				key := strings.ReplaceAll(v.Key, "[hyperbackup,1]", "[{#PRODUCT},{#TASKID}]")
				if !keys[key] {
					t.Fatalf("sender value has no matching template item: %s", v.Key)
				}
			}
		}
	}
}
