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
	DefaultSenderLog   = "/var/packages/synology-activebackup-zabbix/var/cache/sender-log.json"
	DefaultLogDir      = "/var/packages/synology-activebackup-zabbix/var/log"
	DefaultRunDir      = "/var/packages/synology-activebackup-zabbix/var/run"
)

type Config struct {
	Collector CollectorConfig `yaml:"collector" json:"collector"`
	API       APIConfig       `yaml:"api" json:"api"`
	Zabbix    ZabbixConfig    `yaml:"zabbix" json:"zabbix"`
	Products  ProductsConfig  `yaml:"products" json:"products"`
	Logging   LoggingConfig   `yaml:"logging" json:"logging"`
	Paths     PathsConfig     `yaml:"paths" json:"paths"`
	Privacy   PrivacyConfig   `yaml:"privacy" json:"privacy"`
}

type CollectorConfig struct {
	IntervalSeconds int `yaml:"interval_seconds" json:"interval_seconds"`
	MaxAgeHours     int `yaml:"max_age_hours" json:"max_age_hours"`
}

type APIConfig struct {
	Enabled bool   `yaml:"enabled" json:"enabled"`
	Bind    string `yaml:"bind" json:"bind"`
	Port    int    `yaml:"port" json:"port"`
	Token   string `yaml:"token" json:"token"`
}

type ZabbixConfig struct {
	Mode   string             `yaml:"mode" json:"mode"`
	Sender ZabbixSenderConfig `yaml:"sender" json:"sender"`
}

type ZabbixSenderConfig struct {
	Server         string `yaml:"server" json:"server"`
	Port           int    `yaml:"port" json:"port"`
	Host           string `yaml:"host" json:"host"`
	TimeoutSeconds int    `yaml:"timeout_seconds" json:"timeout_seconds"`
	ChunkSize      int    `yaml:"chunk_size" json:"chunk_size"`
	TLS            string `yaml:"tls" json:"tls"`
	ServerName     string `yaml:"server_name" json:"server_name"`
	CAFile         string `yaml:"ca_file" json:"ca_file"`
	CertFile       string `yaml:"cert_file" json:"cert_file"`
	KeyFile        string `yaml:"key_file" json:"key_file"`
	PSKIdentity    string `yaml:"psk_identity" json:"psk_identity"`
	PSK            string `yaml:"psk" json:"psk"`
}

type ProductsConfig struct {
	ActiveBackupBusiness ProductConfig `yaml:"active_backup_business" json:"active_backup_business"`
	ActiveBackupM365     ProductConfig `yaml:"active_backup_m365" json:"active_backup_m365"`
}

type ProductConfig struct {
	Enabled   bool     `yaml:"enabled" json:"enabled"`
	ScanPaths []string `yaml:"scan_paths" json:"scan_paths"`
}

type LoggingConfig struct {
	Level string `yaml:"level" json:"level"`
}

type PathsConfig struct {
	PackageRoot   string `yaml:"package_root" json:"package_root"`
	CacheFile     string `yaml:"cache_file" json:"cache_file"`
	SenderLogFile string `yaml:"sender_log_file" json:"sender_log_file"`
	LogDir        string `yaml:"log_dir" json:"log_dir"`
	RunDir        string `yaml:"run_dir" json:"run_dir"`
}

type PrivacyConfig struct {
	RedactNames bool `yaml:"redact_names" json:"redact_names"`
}

func Default() Config {
	return Config{
		Collector: CollectorConfig{
			IntervalSeconds: 300,
			MaxAgeHours:     30,
		},
		API: APIConfig{
			Enabled: true,
			Bind:    "0.0.0.0",
			Port:    9876,
		},
		Zabbix: ZabbixConfig{
			Mode: "api",
			Sender: ZabbixSenderConfig{
				Port:           10051,
				TimeoutSeconds: 30,
				ChunkSize:      250,
				TLS:            "none",
			},
		},
		Products: ProductsConfig{
			ActiveBackupBusiness: ProductConfig{
				Enabled: true,
				ScanPaths: []string{
					"/volume*/@ActiveBackup",
					"/volume*/ActiveBackupforBusiness",
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
			PackageRoot:   DefaultPackageRoot,
			CacheFile:     DefaultCachePath,
			SenderLogFile: DefaultSenderLog,
			LogDir:        DefaultLogDir,
			RunDir:        DefaultRunDir,
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
	return Write(path, Default())
}

func Write(path string, cfg Config) error {
	if path == "" {
		path = DefaultConfigPath
	}
	cfg.normalize()
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
	if c.API.Bind == "" {
		c.API.Bind = "0.0.0.0"
	}
	if c.API.Port <= 0 {
		c.API.Port = 9876
	}
	if c.Zabbix.Mode == "" {
		c.Zabbix.Mode = "api"
	}
	if c.Zabbix.Sender.Port <= 0 {
		c.Zabbix.Sender.Port = 10051
	}
	if c.Zabbix.Sender.TimeoutSeconds <= 0 {
		c.Zabbix.Sender.TimeoutSeconds = 30
	}
	if c.Zabbix.Sender.ChunkSize <= 0 {
		c.Zabbix.Sender.ChunkSize = 250
	}
	if c.Zabbix.Sender.TLS == "" {
		c.Zabbix.Sender.TLS = "none"
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
	if c.Paths.SenderLogFile == "" {
		c.Paths.SenderLogFile = DefaultSenderLog
	}
	if c.Paths.LogDir == "" {
		c.Paths.LogDir = DefaultLogDir
	}
	if c.Paths.RunDir == "" {
		c.Paths.RunDir = DefaultRunDir
	}
	defaults := Default()
	if len(c.Products.ActiveBackupBusiness.ScanPaths) == 0 {
		c.Products.ActiveBackupBusiness.ScanPaths = defaults.Products.ActiveBackupBusiness.ScanPaths
	}
	if len(c.Products.ActiveBackupM365.ScanPaths) == 0 {
		c.Products.ActiveBackupM365.ScanPaths = defaults.Products.ActiveBackupM365.ScanPaths
	}
}

func (c Config) EnsureDirs() error {
	for _, dir := range []string{
		filepath.Dir(c.Paths.CacheFile),
		filepath.Dir(c.Paths.SenderLogFile),
		c.Paths.LogDir,
		c.Paths.RunDir,
	} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
	}
	return nil
}
