package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/pthoelken/synology-activebackup-zabbixmonitor/internal/abb"
	"github.com/pthoelken/synology-activebackup-zabbixmonitor/internal/apiserver"
	"github.com/pthoelken/synology-activebackup-zabbixmonitor/internal/collector"
	"github.com/pthoelken/synology-activebackup-zabbixmonitor/internal/config"
	"github.com/pthoelken/synology-activebackup-zabbixmonitor/internal/dsmcgi"
	"github.com/pthoelken/synology-activebackup-zabbixmonitor/internal/logging"
	"github.com/pthoelken/synology-activebackup-zabbixmonitor/internal/m365"
	"github.com/pthoelken/synology-activebackup-zabbixmonitor/internal/zabbix"
)

var version = "0.1.15"

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	global := flag.NewFlagSet("synology-activebackup-zabbix", flag.ContinueOnError)
	global.SetOutput(os.Stderr)
	configPath := global.String("config", config.DefaultConfigPath, "configuration file")
	showVersion := global.Bool("version", false, "print version")
	if err := global.Parse(args); err != nil {
		return 2
	}
	if *showVersion {
		fmt.Println(version)
		return 0
	}
	if global.NArg() == 0 {
		usage()
		return 2
	}

	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	logger, closeLogger, err := logging.New(cfg.Logging.Level, cfg.Paths.LogDir, false)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	defer closeLogger()

	cmd := global.Arg(0)
	rest := global.Args()[1:]
	switch cmd {
	case "service":
		return runService(cfg, *configPath)
	case "discovery":
		return cmdDiscovery(rest, cfg, logger)
	case "status":
		return cmdStatus(rest, cfg, logger)
	case "job":
		return cmdJob(rest, cfg, logger)
	case "health":
		return cmdHealth(rest, cfg, logger)
	case "summary":
		return cmdSummary(rest, cfg, logger)
	case "detect":
		return cmdDetect(rest, cfg, logger)
	case "collect":
		return cmdCollect(rest, cfg, logger)
	case "send":
		return cmdSend(rest, cfg, logger)
	case "configure":
		return cmdConfigure(rest, cfg, *configPath)
	case "dsm-cgi":
		return dsmcgi.Run(cfg)
	case "write-default-config":
		if err := config.WriteDefault(*configPath); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		return 0
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n", cmd)
		usage()
		return 2
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: synology-activebackup-zabbix [--config path] <service|discovery|status|job|health|summary|detect|collect|send|configure|dsm-cgi>")
}

func runService(cfg config.Config, configPath string) int {
	if err := cfg.EnsureDirs(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	logger, closeLogger, err := logging.New(cfg.Logging.Level, cfg.Paths.LogDir, true)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	defer closeLogger()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	store := collector.NewStore()
	if snapshot, err := collector.ReadCache(cfg.Paths.CacheFile); err == nil {
		store.Set(snapshot)
	}

	collectAndStore(ctx, cfg, store, logger)
	if cfg.API.Enabled {
		api := apiserver.New(cfg, configPath, store, logger)
		go func() {
			if err := api.ListenAndServe(ctx); err != nil {
				logger.Error("api server stopped", "error", err)
				stop()
			}
		}()
	}
	ticker := time.NewTicker(time.Duration(cfg.Collector.IntervalSeconds) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			logger.Info("service stopped")
			return 0
		case <-ticker.C:
			collectAndStore(ctx, cfg, store, logger)
		}
	}
}

func collectAndStore(ctx context.Context, cfg config.Config, store *collector.Store, logger *slog.Logger) {
	snapshot := collectSnapshot(ctx, cfg, logger)
	if err := collector.WriteCache(cfg.Paths.CacheFile, snapshot); err != nil {
		logger.Error("could not write cache", "error", err)
	}
	store.Set(snapshot)
	if strings.EqualFold(cfg.Zabbix.Mode, "sender") {
		report, err := zabbix.SendSnapshot(ctx, cfg, snapshot)
		if err != nil {
			logger.Error("zabbix sender failed", "error", err)
		} else if report.Values > 0 {
			logger.Info("zabbix sender finished", "values", report.Values, "chunks", report.Chunks, "info", strings.Join(report.Infos, " | "))
		}
	}
	logger.Info("collection finished", "jobs", len(snapshot.Jobs), "errors", len(snapshot.Errors), "db_missing", strings.Join(snapshot.Health.DBMissing, ","))
}

func cmdDiscovery(args []string, cfg config.Config, logger *slog.Logger) int {
	fs := flag.NewFlagSet("discovery", flag.ContinueOnError)
	product := fs.String("product", "", "filter product: abb or m365")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	snapshot, err := loadOrCollect(cfg, logger)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	data, err := zabbix.DiscoveryJSON(snapshot, *product)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	fmt.Println(string(data))
	return 0
}

func cmdStatus(args []string, cfg config.Config, logger *slog.Logger) int {
	fs := flag.NewFlagSet("status", flag.ContinueOnError)
	fresh := fs.Bool("fresh", false, "collect before printing")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	var snapshot collector.Snapshot
	var err error
	if *fresh {
		snapshot = collectSnapshot(context.Background(), cfg, logger)
	} else {
		snapshot, err = loadOrCollect(cfg, logger)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
	}
	printJSON(snapshot)
	return 0
}

func cmdCollect(args []string, cfg config.Config, logger *slog.Logger) int {
	fs := flag.NewFlagSet("collect", flag.ContinueOnError)
	write := fs.Bool("write-cache", true, "write status cache")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	snapshot := collectSnapshot(context.Background(), cfg, logger)
	if *write {
		if err := collector.WriteCache(cfg.Paths.CacheFile, snapshot); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
	}
	printJSON(snapshot)
	return 0
}

func cmdSend(args []string, cfg config.Config, logger *slog.Logger) int {
	fs := flag.NewFlagSet("send", flag.ContinueOnError)
	fresh := fs.Bool("fresh", false, "collect before sending")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	var snapshot collector.Snapshot
	var err error
	if *fresh {
		snapshot = collectSnapshot(context.Background(), cfg, logger)
		if err := collector.WriteCache(cfg.Paths.CacheFile, snapshot); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
	} else {
		snapshot, err = loadOrCollect(cfg, logger)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
	}
	report, err := zabbix.SendSnapshot(context.Background(), cfg, snapshot)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	printJSON(report)
	return 0
}

func cmdJob(args []string, cfg config.Config, logger *slog.Logger) int {
	fs := flag.NewFlagSet("job", flag.ContinueOnError)
	product := fs.String("product", "", "product: abb or m365")
	taskID := fs.String("task-id", "", "task id")
	field := fs.String("field", "", "field for Zabbix item")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *product == "" || *taskID == "" {
		fmt.Fprintln(os.Stderr, "job requires --product and --task-id")
		return 2
	}
	snapshot, err := loadOrCollect(cfg, logger)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	job, found := zabbix.FindJob(snapshot, *product, *taskID)
	if !found {
		if *field == "status" || *field == "" {
			fmt.Println(collector.StatusNoData)
			return 0
		}
		fmt.Println(0)
		return 0
	}
	if *field == "" {
		printJSON(job)
		return 0
	}
	value, err := zabbix.JobField(job, *field)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	fmt.Println(value)
	return 0
}

func cmdHealth(args []string, cfg config.Config, logger *slog.Logger) int {
	fs := flag.NewFlagSet("health", flag.ContinueOnError)
	field := fs.String("field", "json", "health field")
	product := fs.String("product", "", "product filter")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	snapshot, err := loadOrCollect(cfg, logger)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	value, err := zabbix.HealthField(snapshot, *field, *product)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	fmt.Println(value)
	return 0
}

func cmdSummary(args []string, cfg config.Config, logger *slog.Logger) int {
	fs := flag.NewFlagSet("summary", flag.ContinueOnError)
	field := fs.String("field", "total", "summary field")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	snapshot, err := loadOrCollect(cfg, logger)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	value, err := zabbix.SummaryField(snapshot, *field)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	fmt.Println(value)
	return 0
}

func cmdDetect(args []string, cfg config.Config, logger *slog.Logger) int {
	fs := flag.NewFlagSet("detect", flag.ContinueOnError)
	if err := fs.Parse(args); err != nil {
		return 2
	}
	snapshot := collectSnapshot(context.Background(), cfg, logger)
	printJSON(struct {
		Sources []collector.Source `json:"sources"`
		Errors  []string           `json:"errors,omitempty"`
	}{Sources: snapshot.Sources, Errors: snapshot.Errors})
	return 0
}

func cmdConfigure(args []string, cfg config.Config, configPath string) int {
	fs := flag.NewFlagSet("configure", flag.ContinueOnError)
	apiToken := fs.String("api-token", "", "API bearer token")
	apiBind := fs.String("api-bind", "", "API bind address")
	apiPort := fs.String("api-port", "", "API port")
	printToken := fs.Bool("print-token", false, "print resulting API token")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	if *apiBind != "" {
		cfg.API.Bind = *apiBind
	}
	if *apiPort != "" {
		port, err := strconv.Atoi(*apiPort)
		if err != nil || port <= 0 || port > 65535 {
			fmt.Fprintln(os.Stderr, "api port must be between 1 and 65535")
			return 2
		}
		cfg.API.Port = port
	}
	if *apiToken != "" {
		cfg.API.Token = strings.TrimSpace(*apiToken)
	}
	if cfg.API.Token == "" {
		token, err := config.GenerateAPIToken()
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		cfg.API.Token = token
	}
	cfg.API.Enabled = true
	if err := config.Write(configPath, cfg); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if *printToken {
		fmt.Println(cfg.API.Token)
	}
	return 0
}

func loadOrCollect(cfg config.Config, logger *slog.Logger) (collector.Snapshot, error) {
	snapshot, err := collector.ReadCache(cfg.Paths.CacheFile)
	if err == nil {
		return snapshot, nil
	}
	snapshot = collectSnapshot(context.Background(), cfg, logger)
	_ = collector.WriteCache(cfg.Paths.CacheFile, snapshot)
	return snapshot, nil
}

func collectSnapshot(ctx context.Context, cfg config.Config, logger *slog.Logger) collector.Snapshot {
	now := time.Now()
	var jobs []collector.Job
	var sources []collector.Source
	var errors []string
	enabledProducts := map[string]bool{}

	if cfg.Products.ActiveBackupBusiness.Enabled {
		enabledProducts[collector.ProductABB] = true
		result := abb.Collector{
			ScanPaths: cfg.Products.ActiveBackupBusiness.ScanPaths,
			Logger:    logger,
		}.Collect(ctx, now)
		jobs = append(jobs, result.Jobs...)
		sources = append(sources, result.Sources...)
		errors = appendErrors(errors, result.Errors)
	}
	if cfg.Products.ActiveBackupM365.Enabled {
		enabledProducts[collector.ProductM365] = true
		result := m365.Collector{
			ScanPaths:   cfg.Products.ActiveBackupM365.ScanPaths,
			RedactNames: cfg.Privacy.RedactNames,
			Logger:      logger,
		}.Collect(ctx, now)
		jobs = append(jobs, result.Jobs...)
		sources = append(sources, result.Sources...)
		errors = appendErrors(errors, result.Errors)
	}

	sort.Slice(jobs, func(i, j int) bool {
		if jobs[i].Product == jobs[j].Product {
			return jobs[i].TaskID < jobs[j].TaskID
		}
		return jobs[i].Product < jobs[j].Product
	})

	dbMissing := missingProducts(enabledProducts, sources)
	health := collector.Health{
		OK:              len(errors) == 0 && len(dbMissing) == 0,
		CollectorErrors: errors,
		DBMissing:       dbMissing,
		JobCount:        len(jobs),
		CollectedUnix:   now.Unix(),
	}

	return collector.Snapshot{
		CollectedAt: now,
		Health:      health,
		Jobs:        jobs,
		Sources:     sources,
		Errors:      errors,
	}
}

func appendErrors(out []string, errs []error) []string {
	for _, err := range errs {
		if err != nil {
			out = append(out, err.Error())
		}
	}
	return out
}

func missingProducts(enabled map[string]bool, sources []collector.Source) []string {
	found := map[string]bool{}
	for _, source := range sources {
		if source.Found && source.Error == "" {
			found[source.Product] = true
		}
	}
	var missing []string
	for product := range enabled {
		if !found[product] {
			missing = append(missing, product)
		}
	}
	sort.Strings(missing)
	return missing
}

func printJSON(value any) {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	_ = enc.Encode(value)
}
