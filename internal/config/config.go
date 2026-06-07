package config

import (
	"errors"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

const (
	PackageName        = "synology-activebackup-zabbix"
	BinaryName         = "synology-activebackup-zabbix"
	DefaultPackageRoot = "/var/packages/synology-activebackup-zabbix"
	DefaultConfigPath  = "/var/packages/synology-activebackup-zabbix/etc/config.yml"
	DefaultCachePath   = "/var/packages/synology-activebackup-zabbix/var/cache/status.json"
	DefaultLogDir      = "/var/packages/synology-activebackup-zabbix/var/log"
	DefaultRunDir      = "/var/packages/synology-activebackup-zabbix/var/run"
)

type Config struct {
	Collector CollectorConfig `yaml:"collector"`
	Zabbix    ZabbixConfig    `yaml:"zabbix"`
	HTTP      HTTPConfig      `yaml:"http"`
	Products  ProductsConfig  `yaml:"products"`
	Logging   LoggingConfig   `yaml:"logging"`
	Paths     PathsConfig     `yaml:"paths"`
	Privacy   PrivacyConfig   `yaml:"privacy"`
}

type CollectorConfig struct {
	IntervalSeconds int `yaml:"interval_seconds"`
	MaxAgeHours     int `yaml:"max_age_hours"`
}

type ZabbixConfig struct {
	Mode              string `yaml:"mode"`
	UserParameterFile string `yaml:"userparameter_file"`
}

type HTTPConfig struct {
	Enabled bool   `yaml:"enabled"`
	Bind    string `yaml:"bind"`
	Port    int    `yaml:"port"`
}

type ProductsConfig struct {
	ActiveBackupBusiness ProductConfig `yaml:"active_backup_business"`
	ActiveBackupM365     ProductConfig `yaml:"active_backup_m365"`
}

type ProductConfig struct {
	Enabled   bool     `yaml:"enabled"`
	ScanPaths []string `yaml:"scan_paths"`
}

type LoggingConfig struct {
	Level string `yaml:"level"`
}

type PathsConfig struct {
	PackageRoot string `yaml:"package_root"`
	CacheFile   string `yaml:"cache_file"`
	LogDir      string `yaml:"log_dir"`
	RunDir      string `yaml:"run_dir"`
}

type PrivacyConfig struct {
	RedactNames bool `yaml:"redact_names"`
}

func Default() Config {
	return Config{
		Collector: CollectorConfig{
			IntervalSeconds: 300,
			MaxAgeHours:     30,
		},
		Zabbix: ZabbixConfig{
			Mode:              "agent",
			UserParameterFile: "/etc/zabbix/zabbix_agent2.d/synology_activebackup.conf",
		},
		HTTP: HTTPConfig{
			Enabled: true,
			Bind:    "127.0.0.1",
			Port:    9876,
		},
		Products: ProductsConfig{
			ActiveBackupBusiness: ProductConfig{
				Enabled: true,
				ScanPaths: []string{
					"/volume*/@ActiveBackup",
					"/volume*/ActiveBackupforBusiness",
					"/volume*/@ActiveBackup*/db",
				},
			},
			ActiveBackupM365: ProductConfig{
				Enabled: true,
				ScanPaths: []string{
					"/volume*/@ActiveBackup-Office365/db",
				},
			},
		},
		Logging: LoggingConfig{
			Level: "info",
		},
		Paths: PathsConfig{
			PackageRoot: DefaultPackageRoot,
			CacheFile:   DefaultCachePath,
			LogDir:      DefaultLogDir,
			RunDir:      DefaultRunDir,
		},
		Privacy: PrivacyConfig{
			RedactNames: true,
		},
	}
}

func Load(path string) (Config, error) {
	cfg := Default()
	if path == "" {
		path = DefaultConfigPath
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			cfg.normalize()
			return cfg, nil
		}
		return cfg, err
	}
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return cfg, err
	}
	cfg.normalize()
	return cfg, nil
}

func WriteDefault(path string) error {
	if path == "" {
		path = DefaultConfigPath
	}
	cfg := Default()
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

func (c *Config) normalize() {
	if c.Collector.IntervalSeconds <= 0 {
		c.Collector.IntervalSeconds = 300
	}
	if c.Collector.MaxAgeHours <= 0 {
		c.Collector.MaxAgeHours = 30
	}
	if c.Zabbix.Mode == "" {
		c.Zabbix.Mode = "agent"
	}
	if c.Zabbix.UserParameterFile == "" {
		c.Zabbix.UserParameterFile = "/etc/zabbix/zabbix_agent2.d/synology_activebackup.conf"
	}
	if c.HTTP.Bind == "" {
		c.HTTP.Bind = "127.0.0.1"
	}
	if c.HTTP.Port == 0 {
		c.HTTP.Port = 9876
	}
	if c.Logging.Level == "" {
		c.Logging.Level = "info"
	}
	if c.Paths.PackageRoot == "" {
		c.Paths.PackageRoot = DefaultPackageRoot
	}
	if c.Paths.CacheFile == "" {
		c.Paths.CacheFile = DefaultCachePath
	}
	if c.Paths.LogDir == "" {
		c.Paths.LogDir = DefaultLogDir
	}
	if c.Paths.RunDir == "" {
		c.Paths.RunDir = DefaultRunDir
	}
	if len(c.Products.ActiveBackupBusiness.ScanPaths) == 0 {
		c.Products.ActiveBackupBusiness.ScanPaths = Default().Products.ActiveBackupBusiness.ScanPaths
	}
	if len(c.Products.ActiveBackupM365.ScanPaths) == 0 {
		c.Products.ActiveBackupM365.ScanPaths = Default().Products.ActiveBackupM365.ScanPaths
	}
}

func (c Config) EnsureDirs() error {
	for _, dir := range []string{
		filepath.Dir(c.Paths.CacheFile),
		c.Paths.LogDir,
		c.Paths.RunDir,
	} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
	}
	return nil
}
