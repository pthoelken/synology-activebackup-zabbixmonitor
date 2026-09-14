package hyperbackup

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"syscall"
	"time"
)

// APIConfig selects authenticated HTTPS when URL is set. The local command is
// retained for installations where the package account has local API access.
type APIConfig struct {
	InsecureSkipVerify bool
	URL                string `yaml:"url" json:"url"`
	Username           string `yaml:"username" json:"username"`
	Password           string `yaml:"password" json:"password"`
}

type apiClient struct {
	base string
	sid  string
	http *http.Client
}

func newAPIClient(cfg APIConfig) (*apiClient, error) {
	u, err := url.Parse(cfg.URL)
	if err != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil || u.RawQuery != "" || u.ForceQuery || u.RawPath != "" || u.Fragment != "" || (u.Path != "" && u.Path != "/") {
		return nil, errors.New("Hyper Backup API URL must be an HTTPS origin, for example https://nas.example.com:5001")
	}
	if cfg.Username == "" || cfg.Password == "" {
		return nil, errors.New("Hyper Backup HTTPS requires a DSM username and password")
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig = &tls.Config{
		MinVersion: tls.VersionTLS12,
		// Explicit per-product opt-in for NAS installations with untrusted or
		// mismatched certificates. Defaults to false; never changes global TLS.
		InsecureSkipVerify: cfg.InsecureSkipVerify,
	}
	return &apiClient{base: strings.TrimRight(cfg.URL, "/"), http: &http.Client{
		Transport: transport,
		Timeout:   15 * time.Second,
		// Never forward DSM credentials or sessions to redirects.
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}}, nil
}

func (c *apiClient) login(ctx context.Context, cfg APIConfig) error {
	data, err := c.post(ctx, "/webapi/auth.cgi", url.Values{
		"api": {"SYNO.API.Auth"}, "version": {"6"}, "method": {"login"},
		"account": {cfg.Username}, "passwd": {cfg.Password}, "format": {"sid"},
	})
	if err != nil {
		return err
	}
	var response struct {
		Success bool `json:"success"`
		Data    struct {
			SID string `json:"sid"`
		} `json:"data"`
		Error struct {
			Code int `json:"code"`
		} `json:"error"`
	}
	if json.Unmarshal(data, &response) != nil {
		return errors.New("invalid JSON from DSM login")
	}
	if !response.Success || response.Data.SID == "" {
		return dsmLoginError(response.Error.Code)
	}
	c.sid = response.Data.SID
	return nil
}

func (c *apiClient) close() {
	if c.sid != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		values := url.Values{"api": {"SYNO.API.Auth"}, "version": {"6"}, "method": {"logout"}, "_sid": {c.sid}}
		_, _ = c.post(ctx, "/webapi/auth.cgi", values)
	}
	c.http.CloseIdleConnections()
}

func (c *apiClient) run(ctx context.Context, args ...string) ([]byte, error) {
	values := url.Values{"_sid": {c.sid}}
	for _, arg := range args {
		if key, value, ok := strings.Cut(arg, "="); ok {
			values.Set(key, value)
		}
	}
	return c.post(ctx, "/webapi/entry.cgi", values)
}

func dsmLoginError(code int) error {
	switch code {
	case 400:
		return errors.New("DSM login failed (code 400): account does not exist or password is incorrect")
	case 401:
		return errors.New("DSM login failed (code 401): account is disabled")
	case 402:
		return errors.New("DSM login failed (code 402): account is not permitted to use this DSM API")
	case 403, 406:
		return fmt.Errorf("DSM login failed (code %d): two-factor authentication is required and cannot be completed by the collector", code)
	case 404:
		return errors.New("DSM login failed (code 404): two-factor authentication failed")
	case 407:
		return errors.New("DSM login failed (code 407): source address is blocked by DSM")
	case 408, 409:
		return fmt.Errorf("DSM login failed (code %d): account password has expired", code)
	case 410:
		return errors.New("DSM login failed (code 410): account password must be changed")
	default:
		return fmt.Errorf("DSM login failed (code %d)", code)
	}
}

func (c *apiClient) post(ctx context.Context, path string, values url.Values) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.base+path, strings.NewReader(values.Encode()))
	if err != nil {
		return nil, errors.New("could not create DSM HTTPS request")
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	res, err := c.http.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			err = ctx.Err()
		}
		return nil, transportError(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("DSM HTTPS returned HTTP %d", res.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(res.Body, maxOutput+1))
	if err != nil {
		return nil, errors.New("could not read DSM HTTPS response")
	}
	if len(data) > maxOutput {
		return nil, errors.New("DSM API output exceeds 4 MiB")
	}
	return data, nil
}

// Report actionable categories without echoing URLs, certificate identities,
// remote messages or wrapped errors which may contain sensitive values.
func transportError(err error) error {
	prefix := "DSM HTTPS request failed: "
	var authority x509.UnknownAuthorityError
	var hostname x509.HostnameError
	var invalid x509.CertificateInvalidError
	var verification *tls.CertificateVerificationError
	var dns *net.DNSError
	var network net.Error
	var record tls.RecordHeaderError
	switch {
	case errors.Is(err, context.Canceled):
		return errors.New(prefix + "request canceled")
	case errors.As(err, &authority):
		return errors.New(prefix + "TLS certificate issuer is not trusted; configure a DSM certificate whose issuer is trusted by the NAS and serve the complete certificate chain")
	case errors.As(err, &hostname):
		return errors.New(prefix + "TLS certificate does not match the configured hostname or IP address; use a DSM URL covered by the certificate")
	case errors.As(err, &invalid):
		if invalid.Reason == x509.Expired {
			return errors.New(prefix + "TLS certificate is expired or not yet valid; check the NAS clock and certificate validity")
		}
		return errors.New(prefix + "TLS certificate is invalid; check its usage and certificate chain")
	case errors.As(err, &verification):
		return errors.New(prefix + "TLS certificate verification failed; check the DSM certificate and trust chain")
	case errors.As(err, &dns):
		return errors.New(prefix + "DNS lookup failed; check that the configured DSM hostname resolves from the NAS")
	case errors.Is(err, context.DeadlineExceeded):
		return errors.New(prefix + "connection or response timed out; check routing, firewall and DSM HTTPS port")
	case errors.As(err, &network) && network.Timeout():
		return errors.New(prefix + "connection or response timed out; check routing, firewall and DSM HTTPS port")
	case errors.Is(err, syscall.ECONNREFUSED):
		return errors.New(prefix + "connection refused; check the DSM HTTPS port and that DSM is listening on the configured address")
	case errors.Is(err, syscall.ENETUNREACH), errors.Is(err, syscall.EHOSTUNREACH):
		return errors.New(prefix + "network or host unreachable; check NAS routing and firewall")
	case errors.As(err, &record):
		return errors.New(prefix + "invalid TLS response; check that the configured port serves HTTPS")
	case errors.Is(err, io.EOF), errors.Is(err, io.ErrUnexpectedEOF), errors.Is(err, syscall.ECONNRESET):
		return errors.New(prefix + "connection closed unexpectedly; check DSM HTTPS and any reverse proxy")
	default:
		return errors.New(prefix + "transport failure; check DSM HTTPS, connectivity and any reverse proxy")
	}
}
