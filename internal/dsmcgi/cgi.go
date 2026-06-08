package dsmcgi

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/pthoelken/synology-activebackup-zabbixmonitor/internal/config"
)

var allowedPaths = map[string]bool{
	"/ping":       true,
	"/status":     true,
	"/discovery":  true,
	"/health":     true,
	"/summary":    true,
	"/job":        true,
	"/sender-log": true,
	"/config":     true,
}

func Run(cfg config.Config) int {
	if !isDSMUserAuthenticated() {
		writeCGI(http.StatusUnauthorized, "application/json; charset=utf-8", []byte(`{"error":"DSM authentication required"}`))
		return 0
	}

	path := cgiPath()
	if !allowedPaths[path] {
		writeCGI(http.StatusNotFound, "application/json; charset=utf-8", []byte(`{"error":"not found"}`))
		return 0
	}
	if cfg.API.Token == "" {
		writeCGI(http.StatusServiceUnavailable, "application/json; charset=utf-8", []byte(`{"error":"API token is not configured"}`))
		return 0
	}

	req, err := proxyRequest(cfg, path)
	if err != nil {
		writeCGI(http.StatusBadRequest, "application/json; charset=utf-8", []byte(fmt.Sprintf(`{"error":%q}`, err.Error())))
		return 0
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		writeCGI(http.StatusBadGateway, "application/json; charset=utf-8", []byte(fmt.Sprintf(`{"error":%q}`, err.Error())))
		return 0
	}
	defer res.Body.Close()

	body, err := io.ReadAll(io.LimitReader(res.Body, 10<<20))
	if err != nil {
		writeCGI(http.StatusBadGateway, "application/json; charset=utf-8", []byte(fmt.Sprintf(`{"error":%q}`, err.Error())))
		return 0
	}
	contentType := res.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	writeCGI(res.StatusCode, contentType, body)
	return 0
}

func isDSMUserAuthenticated() bool {
	out, err := exec.Command("/usr/syno/synoman/webman/modules/authenticate.cgi").Output()
	if err == nil && strings.TrimSpace(string(out)) != "" {
		return true
	}
	return isDSMWebAPIAuthenticated()
}

func isDSMWebAPIAuthenticated() bool {
	cookie := os.Getenv("HTTP_COOKIE")
	if cookie == "" {
		return false
	}
	synoToken := synoTokenFromQuery()

	scheme := "http"
	port := os.Getenv("SERVER_PORT")
	if strings.EqualFold(os.Getenv("HTTPS"), "on") {
		scheme = "https"
		if port == "" {
			port = "5001"
		}
	} else if port == "" {
		port = "5000"
	}

	query := url.Values{
		"api":     {"SYNO.Core.System"},
		"version": {"3"},
		"method":  {"info"},
	}
	if synoToken != "" {
		query.Set("SynoToken", synoToken)
	}
	u := url.URL{
		Scheme:   scheme,
		Host:     "127.0.0.1:" + port,
		Path:     "/webapi/entry.cgi",
		RawQuery: query.Encode(),
	}
	client := http.Client{
		Timeout: 3 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, // local DSM self-signed certificate
		},
	}
	req, err := http.NewRequest(http.MethodGet, u.String(), nil)
	if err != nil {
		return false
	}
	req.Header.Set("Cookie", cookie)
	if synoToken != "" {
		req.Header.Set("X-SYNO-TOKEN", synoToken)
	}
	res, err := client.Do(req)
	if err != nil {
		return false
	}
	defer res.Body.Close()
	var response struct {
		Success bool `json:"success"`
	}
	if err := json.NewDecoder(io.LimitReader(res.Body, 1<<20)).Decode(&response); err != nil {
		return false
	}
	return response.Success
}

func proxyRequest(cfg config.Config, path string) (*http.Request, error) {
	method := os.Getenv("REQUEST_METHOD")
	if method == "" {
		method = http.MethodGet
	}
	if method != http.MethodGet && method != http.MethodPost && method != http.MethodPut {
		return nil, fmt.Errorf("method %s is not allowed", method)
	}

	u := url.URL{
		Scheme: "http",
		Host:   "127.0.0.1:" + strconv.Itoa(cfg.API.Port),
		Path:   "/api/v1" + path,
	}
	if rawQuery := os.Getenv("QUERY_STRING"); rawQuery != "" {
		query, err := url.ParseQuery(rawQuery)
		if err != nil {
			return nil, err
		}
		query.Del("SynoToken")
		query.Del("synotoken")
		query.Del("t")
		u.RawQuery = query.Encode()
	}

	var body io.Reader
	if method == http.MethodPost || method == http.MethodPut {
		limit := int64(1 << 20)
		if length, err := strconv.ParseInt(os.Getenv("CONTENT_LENGTH"), 10, 64); err == nil && length > 0 && length < limit {
			limit = length
		}
		data, err := io.ReadAll(io.LimitReader(os.Stdin, limit))
		if err != nil {
			return nil, err
		}
		body = bytes.NewReader(data)
	}

	req, err := http.NewRequest(method, u.String(), body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+cfg.API.Token)
	if contentType := os.Getenv("CONTENT_TYPE"); contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	return req, nil
}

func synoTokenFromQuery() string {
	query, err := url.ParseQuery(os.Getenv("QUERY_STRING"))
	if err != nil {
		return ""
	}
	for _, key := range []string{"SynoToken", "synotoken", "synoToken"} {
		if token := strings.TrimSpace(query.Get(key)); token != "" {
			return token
		}
	}
	return ""
}

func cgiPath() string {
	path := os.Getenv("PATH_INFO")
	if path == "" {
		uri := os.Getenv("REQUEST_URI")
		if idx := strings.Index(uri, "api.cgi"); idx >= 0 {
			path = uri[idx+len("api.cgi"):]
			if queryIdx := strings.Index(path, "?"); queryIdx >= 0 {
				path = path[:queryIdx]
			}
		}
	}
	path = "/" + strings.TrimLeft(path, "/")
	if path == "/" {
		return "/status"
	}
	if strings.Contains(path, "..") {
		return ""
	}
	return path
}

func writeCGI(status int, contentType string, body []byte) {
	fmt.Printf("Status: %d %s\r\n", status, http.StatusText(status))
	fmt.Printf("Content-Type: %s\r\n", contentType)
	fmt.Print("Cache-Control: no-store\r\n\r\n")
	_, _ = os.Stdout.Write(body)
}
