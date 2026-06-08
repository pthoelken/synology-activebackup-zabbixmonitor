package zabbix

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	psktls "github.com/jc-lab/go-tls-psk"
	"github.com/pthoelken/synology-activebackup-zabbixmonitor/internal/collector"
	"github.com/pthoelken/synology-activebackup-zabbixmonitor/internal/config"
)

type SenderValue struct {
	Host  string `json:"host"`
	Key   string `json:"key"`
	Value string `json:"value"`
	Clock int64  `json:"clock,omitempty"`
	NS    int    `json:"ns,omitempty"`
}

type SenderReport struct {
	Values       int      `json:"values"`
	Chunks       int      `json:"chunks"`
	Processed    int      `json:"processed,omitempty"`
	Failed       int      `json:"failed,omitempty"`
	Total        int      `json:"total,omitempty"`
	SecondsSpent float64  `json:"seconds_spent,omitempty"`
	Infos        []string `json:"infos,omitempty"`
}

type SenderLog struct {
	Entries []SenderLogEntry `json:"entries"`
}

type SenderLogEntry struct {
	At          time.Time     `json:"at"`
	OK          bool          `json:"ok"`
	Server      string        `json:"server"`
	Port        int           `json:"port"`
	TLS         string        `json:"tls"`
	Host        string        `json:"host"`
	ValuesCount int           `json:"values_count"`
	Chunks      int           `json:"chunks"`
	Processed   int           `json:"processed,omitempty"`
	Failed      int           `json:"failed,omitempty"`
	Total       int           `json:"total,omitempty"`
	Infos       []string      `json:"infos,omitempty"`
	Error       string        `json:"error,omitempty"`
	Values      []SenderValue `json:"values,omitempty"`
}

type senderRequest struct {
	Request string        `json:"request"`
	Data    []SenderValue `json:"data"`
	Clock   int64         `json:"clock,omitempty"`
	NS      int           `json:"ns,omitempty"`
}

type senderResponse struct {
	Response string `json:"response"`
	Info     string `json:"info"`
}

func SendSnapshot(ctx context.Context, cfg config.Config, snapshot collector.Snapshot) (SenderReport, error) {
	entry := SenderLogEntry{
		At:     time.Now(),
		Server: cfg.Zabbix.Sender.Server,
		Port:   cfg.Zabbix.Sender.Port,
		TLS:    normalizedTLSMode(cfg.Zabbix.Sender.TLS),
		Host:   cfg.Zabbix.Sender.Host,
	}
	values, err := SnapshotSenderValues(cfg, snapshot)
	if err != nil {
		entry.Error = err.Error()
		_ = AppendSenderLog(cfg.Paths.SenderLogFile, entry)
		return SenderReport{}, err
	}
	entry.ValuesCount = len(values)
	entry.Values = senderLogValues(values, 200)
	if len(values) == 0 {
		entry.OK = true
		_ = AppendSenderLog(cfg.Paths.SenderLogFile, entry)
		return SenderReport{}, nil
	}
	if entry.Host == "" {
		entry.Host = values[0].Host
	}

	senderCfg := cfg.Zabbix.Sender
	if len(zabbixServers(senderCfg.Server)) == 0 {
		err := fmt.Errorf("zabbix sender server is required")
		entry.Error = err.Error()
		_ = AppendSenderLog(cfg.Paths.SenderLogFile, entry)
		return SenderReport{}, err
	}
	var report SenderReport
	switch strings.ToLower(strings.TrimSpace(senderCfg.TLS)) {
	case "", "none", "plain", "tcp":
		report, err = sendNative(ctx, senderCfg, values, false)
	case "cert", "certificate", "tls":
		report, err = sendNative(ctx, senderCfg, values, true)
	case "psk":
		report, err = sendPSK(ctx, senderCfg, values)
	default:
		err = fmt.Errorf("unsupported zabbix sender tls mode %q", senderCfg.TLS)
	}
	entry.OK = err == nil
	entry.Chunks = report.Chunks
	entry.Processed = report.Processed
	entry.Failed = report.Failed
	entry.Total = report.Total
	entry.Infos = report.Infos
	if err != nil {
		entry.Error = err.Error()
	}
	_ = AppendSenderLog(cfg.Paths.SenderLogFile, entry)
	return report, err
}

func SnapshotSenderValues(cfg config.Config, snapshot collector.Snapshot) ([]SenderValue, error) {
	host := strings.TrimSpace(cfg.Zabbix.Sender.Host)
	if host == "" {
		detected, err := os.Hostname()
		if err != nil {
			return nil, err
		}
		host = detected
	}
	clock := snapshot.CollectedAt.Unix()
	ns := snapshot.CollectedAt.Nanosecond()

	var values []SenderValue
	add := func(key string, value string) {
		values = append(values, SenderValue{
			Host:  host,
			Key:   key,
			Value: value,
			Clock: clock,
			NS:    ns,
		})
	}

	discovery, err := DiscoveryJSON(snapshot, "")
	if err != nil {
		return nil, err
	}
	add("synology.activebackup.discovery", string(discovery))

	health, err := HealthField(snapshot, "ok", "")
	if err != nil {
		return nil, err
	}
	add("synology.activebackup.health", health)
	healthJSON, err := HealthField(snapshot, "json", "")
	if err != nil {
		return nil, err
	}
	add("synology.activebackup.health.json", healthJSON)
	for _, product := range []string{collector.ProductABB, collector.ProductM365} {
		missing, err := HealthField(snapshot, "db_missing", product)
		if err != nil {
			return nil, err
		}
		add(fmt.Sprintf("synology.activebackup.product.db_missing[%s]", product), missing)
	}
	for _, field := range []string{"successful", "failed", "problem", "total"} {
		value, err := SummaryField(snapshot, field)
		if err != nil {
			return nil, err
		}
		add("synology.activebackup.jobs."+field, value)
	}

	for _, job := range snapshot.Jobs {
		for _, field := range []string{
			"status",
			"age",
			"last_success_age",
			"error",
			"runtime",
			"transferred_size",
			"last_end_time",
			"info",
		} {
			value, err := JobField(job, field)
			if err != nil {
				return nil, err
			}
			add(fmt.Sprintf("synology.activebackup.job.%s[%s,%s]", field, job.Product, job.TaskID), value)
		}
	}
	return values, nil
}

func sendNative(ctx context.Context, cfg config.ZabbixSenderConfig, values []SenderValue, useTLS bool) (SenderReport, error) {
	var report SenderReport
	for _, server := range zabbixServers(cfg.Server) {
		chunks := chunkValues(values, cfg.ChunkSize)
		for _, chunk := range chunks {
			result, err := sendChunk(ctx, cfg, server, chunk, func(ctx context.Context, cfg config.ZabbixSenderConfig, server string) (net.Conn, error) {
				return dialZabbix(ctx, cfg, server, useTLS)
			})
			report.Values += len(chunk)
			report.Chunks++
			report.Processed += result.Processed
			report.Failed += result.Failed
			report.Total += result.Total
			report.SecondsSpent += result.SecondsSpent
			if result.Info != "" {
				report.Infos = append(report.Infos, result.Info)
			}
			if err != nil {
				return report, err
			}
		}
	}
	return report, nil
}

func sendPSK(ctx context.Context, cfg config.ZabbixSenderConfig, values []SenderValue) (SenderReport, error) {
	var report SenderReport
	for _, server := range zabbixServers(cfg.Server) {
		for _, chunk := range chunkValues(values, cfg.ChunkSize) {
			result, err := sendChunk(ctx, cfg, server, chunk, dialZabbixPSK)
			report.Values += len(chunk)
			report.Chunks++
			report.Processed += result.Processed
			report.Failed += result.Failed
			report.Total += result.Total
			report.SecondsSpent += result.SecondsSpent
			if result.Info != "" {
				report.Infos = append(report.Infos, result.Info)
			}
			if err != nil {
				return report, err
			}
		}
	}
	return report, nil
}

type senderChunkResult struct {
	Info         string
	Processed    int
	Failed       int
	Total        int
	SecondsSpent float64
}

func sendChunk(ctx context.Context, cfg config.ZabbixSenderConfig, server string, values []SenderValue, dial func(context.Context, config.ZabbixSenderConfig, string) (net.Conn, error)) (senderChunkResult, error) {
	conn, err := dial(ctx, cfg, server)
	if err != nil {
		return senderChunkResult{}, err
	}
	defer conn.Close()

	timeout := time.Duration(cfg.TimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	_ = conn.SetDeadline(time.Now().Add(timeout))

	now := time.Now()
	payload, err := json.Marshal(senderRequest{
		Request: "sender data",
		Data:    values,
		Clock:   now.Unix(),
		NS:      now.Nanosecond(),
	})
	if err != nil {
		return senderChunkResult{}, err
	}
	if _, err := conn.Write(senderPacket(payload)); err != nil {
		return senderChunkResult{}, err
	}
	responsePayload, err := readSenderPacket(conn)
	if err != nil {
		return senderChunkResult{}, err
	}
	var response senderResponse
	if err := json.Unmarshal(responsePayload, &response); err != nil {
		return senderChunkResult{}, err
	}
	result := parseSenderInfo(response.Info)
	if strings.ToLower(response.Response) != "success" {
		return result, fmt.Errorf("zabbix sender response %q: %s", response.Response, response.Info)
	}
	if result.Failed > 0 {
		return result, fmt.Errorf("zabbix rejected %d of %d values: %s. Check the exact Zabbix technical host name, linked sender template/trapper items, and allowed hosts", result.Failed, result.Total, result.Info)
	}
	return result, nil
}

func dialZabbix(ctx context.Context, cfg config.ZabbixSenderConfig, server string, useTLS bool) (net.Conn, error) {
	dialer := net.Dialer{
		Timeout: time.Duration(cfg.TimeoutSeconds) * time.Second,
	}
	address := net.JoinHostPort(server, strconv.Itoa(cfg.Port))
	if !useTLS {
		return dialer.DialContext(ctx, "tcp", address)
	}
	tlsCfg, err := zabbixTLSConfig(cfg, server)
	if err != nil {
		return nil, err
	}
	return tls.DialWithDialer(&dialer, "tcp", address, tlsCfg)
}

func dialZabbixPSK(ctx context.Context, cfg config.ZabbixSenderConfig, server string) (net.Conn, error) {
	identity := strings.TrimSpace(cfg.PSKIdentity)
	if identity == "" {
		return nil, fmt.Errorf("zabbix sender psk_identity is required")
	}
	psk, err := decodeZabbixPSK(cfg.PSK)
	if err != nil {
		return nil, err
	}
	dialer := net.Dialer{
		Timeout: time.Duration(cfg.TimeoutSeconds) * time.Second,
	}
	address := net.JoinHostPort(server, strconv.Itoa(cfg.Port))
	serverName := strings.TrimSpace(cfg.ServerName)
	if serverName == "" {
		serverName = server
	}
	tlsCfg := &psktls.Config{
		MinVersion: psktls.VersionTLS12,
		MaxVersion: psktls.VersionTLS12,
		ServerName: serverName,
		CipherSuites: []uint16{
			psktls.TLS_ECDHE_PSK_WITH_CHACHA20_POLY1305_SHA256,
			psktls.TLS_ECDHE_PSK_WITH_AES_128_CBC_SHA256,
			psktls.TLS_ECDHE_PSK_WITH_AES_128_CBC_SHA,
			psktls.TLS_ECDHE_PSK_WITH_AES_256_CBC_SHA,
			psktls.TLS_ECDHE_PSK_WITH_AES_256_CBC_SHA384,
		},
		Extra: psktls.PSKConfig{
			GetIdentity: func() string {
				return identity
			},
			GetKey: func(_ string) ([]byte, error) {
				return psk, nil
			},
		},
	}
	return psktls.DialWithDialer(&dialer, "tcp", address, tlsCfg)
}

func decodeZabbixPSK(value string) ([]byte, error) {
	clean := strings.Join(strings.Fields(value), "")
	if clean == "" {
		return nil, fmt.Errorf("zabbix sender psk is required")
	}
	if len(clean)%2 != 0 {
		return nil, fmt.Errorf("zabbix sender psk must be an even-length hexadecimal value")
	}
	psk, err := hex.DecodeString(clean)
	if err != nil {
		return nil, fmt.Errorf("zabbix sender psk must be hexadecimal: %w", err)
	}
	if len(psk) < 16 {
		return nil, fmt.Errorf("zabbix sender psk must be at least 16 bytes / 32 hexadecimal characters")
	}
	if len(psk) > 256 {
		return nil, fmt.Errorf("zabbix sender psk must be at most 256 bytes / 512 hexadecimal characters")
	}
	return psk, nil
}

func zabbixTLSConfig(cfg config.ZabbixSenderConfig, server string) (*tls.Config, error) {
	tlsCfg := &tls.Config{
		MinVersion: tls.VersionTLS12,
		ServerName: strings.TrimSpace(cfg.ServerName),
	}
	if tlsCfg.ServerName == "" {
		tlsCfg.ServerName = server
	}
	if cfg.CAFile != "" {
		data, err := os.ReadFile(cfg.CAFile)
		if err != nil {
			return nil, err
		}
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(data) {
			return nil, fmt.Errorf("could not read CA certificates from %s", cfg.CAFile)
		}
		tlsCfg.RootCAs = pool
	}
	if cfg.CertFile != "" || cfg.KeyFile != "" {
		if cfg.CertFile == "" || cfg.KeyFile == "" {
			return nil, fmt.Errorf("cert_file and key_file must be set together")
		}
		cert, err := tls.LoadX509KeyPair(cfg.CertFile, cfg.KeyFile)
		if err != nil {
			return nil, err
		}
		tlsCfg.Certificates = []tls.Certificate{cert}
	}
	return tlsCfg, nil
}

func senderPacket(payload []byte) []byte {
	packet := make([]byte, 13+len(payload))
	copy(packet, []byte{'Z', 'B', 'X', 'D', 1})
	binary.LittleEndian.PutUint64(packet[5:13], uint64(len(payload)))
	copy(packet[13:], payload)
	return packet
}

func parseSenderInfo(info string) senderChunkResult {
	result := senderChunkResult{Info: info}
	for _, part := range strings.Split(info, ";") {
		key, value, ok := strings.Cut(strings.TrimSpace(part), ":")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		switch key {
		case "processed":
			result.Processed, _ = strconv.Atoi(value)
		case "failed":
			result.Failed, _ = strconv.Atoi(value)
		case "total":
			result.Total, _ = strconv.Atoi(value)
		case "seconds spent":
			result.SecondsSpent, _ = strconv.ParseFloat(value, 64)
		}
	}
	return result
}

func AppendSenderLog(path string, entry SenderLogEntry) error {
	if path == "" {
		path = config.DefaultSenderLog
	}
	log, _ := ReadSenderLog(path)
	log.Entries = append([]SenderLogEntry{entry}, log.Entries...)
	if len(log.Entries) > 50 {
		log.Entries = log.Entries[:50]
	}
	data, err := json.MarshalIndent(log, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(configDir(path), 0755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}

func ReadSenderLog(path string) (SenderLog, error) {
	if path == "" {
		path = config.DefaultSenderLog
	}
	var log SenderLog
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return log, nil
		}
		return log, err
	}
	if len(bytes.TrimSpace(data)) == 0 {
		return log, nil
	}
	err = json.Unmarshal(data, &log)
	return log, err
}

func configDir(path string) string {
	if idx := strings.LastIndex(path, "/"); idx >= 0 {
		return path[:idx]
	}
	return "."
}

func senderLogValues(values []SenderValue, limit int) []SenderValue {
	if limit <= 0 || len(values) <= limit {
		return append([]SenderValue(nil), values...)
	}
	return append([]SenderValue(nil), values[:limit]...)
}

func normalizedTLSMode(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" || value == "plain" || value == "tcp" {
		return "none"
	}
	if value == "certificate" || value == "tls" {
		return "cert"
	}
	return value
}

func readSenderPacket(r io.Reader) ([]byte, error) {
	header := make([]byte, 13)
	if _, err := io.ReadFull(r, header); err != nil {
		return nil, err
	}
	if !bytes.Equal(header[:5], []byte{'Z', 'B', 'X', 'D', 1}) {
		return nil, fmt.Errorf("invalid zabbix response header")
	}
	length := binary.LittleEndian.Uint64(header[5:13])
	if length > 64*1024*1024 {
		return nil, fmt.Errorf("zabbix response too large: %d", length)
	}
	payload := make([]byte, int(length))
	_, err := io.ReadFull(r, payload)
	return payload, err
}

func zabbixServers(value string) []string {
	var out []string
	for _, part := range strings.FieldsFunc(value, func(r rune) bool {
		return r == ',' || r == ';'
	}) {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func chunkValues(values []SenderValue, size int) [][]SenderValue {
	if size <= 0 {
		size = 250
	}
	var chunks [][]SenderValue
	for start := 0; start < len(values); start += size {
		end := start + size
		if end > len(values) {
			end = len(values)
		}
		chunks = append(chunks, values[start:end])
	}
	return chunks
}
