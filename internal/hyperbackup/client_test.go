package hyperbackup

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"syscall"
	"testing"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func response(code int, body string) *http.Response {
	return &http.Response{StatusCode: code, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}
}

func TestHTTPSLifecycle(t *testing.T) {
	cfg := APIConfig{URL: "https://nas.example.com:5001", Username: "monitor+user", Password: "p&= word"}
	c, err := newAPIClient(cfg)
	if err != nil {
		t.Fatal(err)
	}
	tr := c.http.Transport.(*http.Transport)
	if tr.TLSClientConfig.InsecureSkipVerify {
		t.Fatal("TLS verification disabled")
	}
	if c.http.CheckRedirect(nil, nil) != http.ErrUseLastResponse {
		t.Fatal("redirects allowed")
	}
	calls := 0
	c.http.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		calls++
		if r.Method != "POST" || r.URL.RawQuery != "" {
			t.Fatal("credentials must not be in URL")
		}
		if err := r.ParseForm(); err != nil {
			t.Fatal(err)
		}
		switch calls {
		case 1:
			if r.URL.Path != "/webapi/auth.cgi" || r.Form.Get("account") != cfg.Username || r.Form.Get("passwd") != cfg.Password || r.Form.Get("format") != "sid" || r.Form.Get("enable_syno_token") != "" || r.Form.Get("session") != "" {
				t.Fatal("incorrect login")
			}
			return response(200, `{"success":true,"data":{"sid":"test-session"}}`), nil
		case 2:
			if r.URL.Path != "/webapi/entry.cgi" || r.Form.Get("_sid") != "test-session" || r.Form.Get("SynoToken") != "" || r.Form.Get("method") != "status" || r.Form.Get("api") != "SYNO.Backup.Task" || r.Form.Get("task_id") != "1" {
				t.Fatal("incorrect task request")
			}
			if r.Form.Get("passwd") != "" {
				t.Fatal("password repeated outside login")
			}
			return response(200, `{"success":true,"data":{}}`), nil
		case 3:
			if r.URL.Path != "/webapi/auth.cgi" || r.Form.Get("method") != "logout" || r.Form.Get("_sid") != "test-session" || r.Form.Get("SynoToken") != "" || r.Form.Get("session") != "" {
				t.Fatal("incorrect logout")
			}
			return response(200, `{"success":true}`), nil
		default:
			t.Fatal("unexpected request")
			return nil, nil
		}
	})
	if err := c.login(context.Background(), cfg); err != nil {
		t.Fatal(err)
	}
	if _, err := c.run(context.Background(), "--exec", "api=SYNO.Backup.Task", "version=1", "method=status", "task_id=1"); err != nil {
		t.Fatal(err)
	}
	c.close()
	if calls != 3 {
		t.Fatalf("%d calls", calls)
	}
}

func TestDSMLoginErrors(t *testing.T) {
	for code, want := range map[int]string{
		400: "password is incorrect", 401: "account is disabled", 402: "not permitted",
		403: "two-factor authentication", 404: "two-factor authentication failed",
		406: "two-factor authentication", 407: "blocked", 408: "expired",
		409: "expired", 410: "must be changed", 499: "code 499",
	} {
		if err := dsmLoginError(code); err == nil || !strings.Contains(err.Error(), want) {
			t.Fatalf("code %d: got %v, want %q", code, err, want)
		}
	}
}

func TestHTTPSFailures(t *testing.T) {
	for _, origin := range []string{"http://nas:5000", "https://user:pass@nas", "https://nas/webapi", "https://nas?password=secret", "https://nas#fragment", "https://"} {
		if _, err := newAPIClient(APIConfig{URL: origin, Username: "u", Password: "p"}); err == nil {
			t.Fatalf("accepted %s", origin)
		}
	}
	if _, err := newAPIClient(APIConfig{URL: "https://nas"}); err == nil {
		t.Fatal("accepted missing credentials")
	}
	for _, tt := range []struct {
		code int
		body string
	}{
		{200, `{"success":false,"error":{"code":403},"secret":"test-password"}`},
		{200, `{"success":true,"data":{}}`}, {200, `invalid test-password`},
		{401, `test-password`}, {302, `test-password`}, {200, strings.Repeat("x", maxOutput+1)},
	} {
		c, err := newAPIClient(APIConfig{URL: "https://nas", Username: "u", Password: "test-password"})
		if err != nil {
			t.Fatal(err)
		}
		c.http.Transport = roundTripFunc(func(*http.Request) (*http.Response, error) { return response(tt.code, tt.body), nil })
		err = c.login(context.Background(), APIConfig{Username: "u", Password: "test-password"})
		if err == nil || strings.Contains(err.Error(), "test-password") {
			t.Fatalf("unsafe error: %v", err)
		}
	}
}

func TestTransportErrorDiagnostics(t *testing.T) {
	for _, tt := range []struct {
		name string
		err  error
		want string
	}{
		{"untrusted issuer", x509.UnknownAuthorityError{Cert: &x509.Certificate{}}, "issuer is not trusted"},
		{"hostname mismatch", x509.HostnameError{Certificate: &x509.Certificate{}, Host: "private-host"}, "does not match"},
		{"expired", x509.CertificateInvalidError{Cert: &x509.Certificate{}, Reason: x509.Expired}, "expired or not yet valid"},
		{"invalid usage", x509.CertificateInvalidError{Cert: &x509.Certificate{}, Reason: x509.IncompatibleUsage}, "certificate is invalid"},
		{"TLS wrapper", &tls.CertificateVerificationError{Err: x509.UnknownAuthorityError{Cert: &x509.Certificate{}}}, "issuer is not trusted"},
		{"other TLS error", &tls.CertificateVerificationError{Err: errors.New("private detail")}, "verification failed"},
		{"DNS", &net.DNSError{Name: "private-host", Err: "private detail", IsNotFound: true}, "DNS lookup failed"},
		{"deadline", context.DeadlineExceeded, "timed out"},
		{"timeout", &net.DNSError{IsTimeout: true}, "DNS lookup failed"},
		{"canceled", context.Canceled, "request canceled"},
		{"refused", &net.OpError{Op: "dial", Net: "tcp", Err: syscall.ECONNREFUSED}, "connection refused"},
		{"unreachable", syscall.EHOSTUNREACH, "unreachable"},
		{"wrong port", tls.RecordHeaderError{}, "invalid TLS response"},
		{"closed", io.EOF, "closed unexpectedly"},
		{"unknown", errors.New("private detail"), "transport failure"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			c, err := newAPIClient(APIConfig{URL: "https://nas.example.com:5001", Username: "u", Password: "test-password"})
			if err != nil {
				t.Fatal(err)
			}
			c.http.Transport = roundTripFunc(func(*http.Request) (*http.Response, error) {
				return nil, &url.Error{Op: "Post", URL: "https://private-host/?secret=test-password", Err: tt.err}
			})
			_, err = c.post(context.Background(), "/webapi/entry.cgi", url.Values{"passwd": {"test-password"}})
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("got %v, want %s", err, tt.want)
			}
			for _, secret := range []string{"private-host", "private detail", "test-password"} {
				if strings.Contains(err.Error(), secret) {
					t.Fatalf("sensitive detail in %v", err)
				}
			}
		})
	}
}

func TestCertificateVerificationOptIn(t *testing.T) {
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"success":true,"data":{"sid":"test-session"}}`)
	}))
	server.Config.ErrorLog = log.New(io.Discard, "", 0)
	server.StartTLS()
	defer server.Close()

	// The test server's self-signed certificate is not trusted by the default
	// client. A later strict client must still reject it after an opt-in client.
	for _, skip := range []bool{false, true, false} {
		cfg := APIConfig{URL: server.URL, Username: "test-user", Password: "test-password", InsecureSkipVerify: skip}
		client, err := newAPIClient(cfg)
		if err != nil {
			t.Fatal(err)
		}
		err = client.login(context.Background(), cfg)
		client.close()
		if skip && err != nil {
			t.Fatalf("explicit opt-in rejected certificate: %v", err)
		}
		if !skip && (err == nil || !strings.Contains(err.Error(), "issuer is not trusted")) {
			t.Fatalf("strict client did not reject untrusted certificate: %v", err)
		}
	}
}
