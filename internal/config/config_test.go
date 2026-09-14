package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestHyperBackupUpgradeAndRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yml")
	if err := os.WriteFile(path, []byte("products:\n  active_backup_business:\n    enabled: true\n  active_backup_m365:\n    enabled: false\n"), 0600); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Products.HyperBackup.Enabled || cfg.Products.HyperBackup.InsecureSkipVerify || !cfg.Products.ActiveBackupBusiness.Enabled || cfg.Products.ActiveBackupM365.Enabled {
		t.Fatal("upgrade changed product selection")
	}
	cfg.Products.HyperBackup.Enabled = true
	cfg.Products.HyperBackup.InsecureSkipVerify = true
	cfg.Products.HyperBackup.APIURL = "https://nas.example.com:5001"
	cfg.Products.HyperBackup.Username = "monitor"
	cfg.Products.HyperBackup.Password = "test-password"
	if err := Write(path, cfg); err != nil {
		t.Fatal(err)
	}
	cfg, err = Load(path)
	if err != nil || !cfg.Products.HyperBackup.Enabled || !cfg.Products.HyperBackup.InsecureSkipVerify || cfg.Products.HyperBackup.Password != "test-password" || cfg.Products.HyperBackup.APIURL != "https://nas.example.com:5001" {
		t.Fatalf("roundtrip: %+v %v", cfg.Products, err)
	}
}

func TestConfigSecretsPermissions(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yml")
	if err := os.WriteFile(path, []byte("old"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := Write(path, Default()); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0600 {
		t.Fatalf("config permissions: %o", info.Mode().Perm())
	}
}
