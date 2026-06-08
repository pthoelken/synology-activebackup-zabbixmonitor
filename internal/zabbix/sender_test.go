package zabbix

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/binary"
	"encoding/pem"
	"math/big"
	"net"
	"testing"
	"time"

	psktls "github.com/jc-lab/go-tls-psk"
	"github.com/pthoelken/synology-activebackup-zabbixmonitor/internal/collector"
	"github.com/pthoelken/synology-activebackup-zabbixmonitor/internal/config"
)

func TestSenderPacket(t *testing.T) {
	payload := []byte(`{"request":"sender data"}`)
	packet := senderPacket(payload)
	if !bytes.Equal(packet[:5], []byte{'Z', 'B', 'X', 'D', 1}) {
		t.Fatalf("unexpected header: %v", packet[:5])
	}
	if got := binary.LittleEndian.Uint64(packet[5:13]); got != uint64(len(payload)) {
		t.Fatalf("unexpected length: %d", got)
	}
	if !bytes.Equal(packet[13:], payload) {
		t.Fatalf("payload mismatch")
	}
}

func TestSnapshotSenderValues(t *testing.T) {
	end := time.Unix(1710000000, 0)
	cfg := config.Default()
	cfg.Zabbix.Sender.Host = "NAS-HOST"
	snapshot := collector.Snapshot{
		CollectedAt: end,
		Health: collector.Health{
			OK:            true,
			JobCount:      1,
			CollectedUnix: end.Unix(),
		},
		Jobs: []collector.Job{
			{
				Product:         collector.ProductM365,
				TaskID:          "42",
				JobName:         "M365 Backup",
				Status:          collector.StatusOK,
				EndTime:         &end,
				LastSuccessTime: &end,
				LastEndUnix:     end.Unix(),
				TransferredSize: 12345,
				HasData:         true,
			},
		},
		Sources: []collector.Source{
			{Product: collector.ProductM365, Found: true},
		},
	}

	values, err := SnapshotSenderValues(cfg, snapshot)
	if err != nil {
		t.Fatal(err)
	}
	keys := map[string]string{}
	for _, value := range values {
		if value.Host != "NAS-HOST" {
			t.Fatalf("unexpected host %q", value.Host)
		}
		keys[value.Key] = value.Value
	}
	for _, key := range []string{
		"synology.activebackup.discovery",
		"synology.activebackup.health",
		"synology.activebackup.jobs.total",
		"synology.activebackup.job.status[m365,42]",
		"synology.activebackup.job.transferred_size[m365,42]",
		"synology.activebackup.job.info[m365,42]",
	} {
		if _, ok := keys[key]; !ok {
			t.Fatalf("missing key %s", key)
		}
	}
	if keys["synology.activebackup.job.transferred_size[m365,42]"] != "12345" {
		t.Fatalf("unexpected transferred size %q", keys["synology.activebackup.job.transferred_size[m365,42]"])
	}
}

func TestDecodeZabbixPSK(t *testing.T) {
	valid := "00112233445566778899aabbccddeeff"
	psk, err := decodeZabbixPSK(valid)
	if err != nil {
		t.Fatalf("valid psk failed: %v", err)
	}
	if len(psk) != 16 {
		t.Fatalf("unexpected psk length: %d", len(psk))
	}

	for name, value := range map[string]string{
		"empty": "",
		"short": "001122",
		"odd":   "001",
		"bad":   "00112233445566778899aabbccddeefg",
	} {
		if _, err := decodeZabbixPSK(value); err == nil {
			t.Fatalf("%s psk unexpectedly succeeded", name)
		}
	}
}

func TestParseSenderInfo(t *testing.T) {
	result := parseSenderInfo("processed: 0; failed: 41; total: 41; seconds spent: 0.000346")
	if result.Processed != 0 || result.Failed != 41 || result.Total != 41 {
		t.Fatalf("unexpected result: %#v", result)
	}
	if result.SecondsSpent == 0 {
		t.Fatalf("seconds spent was not parsed: %#v", result)
	}
}

func TestPSKHandshakeWithoutServerCertificate(t *testing.T) {
	clientConn, serverConn := net.Pipe()
	defer clientConn.Close()
	defer serverConn.Close()
	deadline := time.Now().Add(5 * time.Second)
	_ = clientConn.SetDeadline(deadline)
	_ = serverConn.SetDeadline(deadline)

	psk := []byte("0123456789abcdef")
	cert := testPSKServerCertificate(t)
	server := psktls.Server(serverConn, &psktls.Config{
		Certificates: []psktls.Certificate{cert},
		MinVersion:   psktls.VersionTLS12,
		MaxVersion:   psktls.VersionTLS12,
		CipherSuites: []uint16{psktls.TLS_ECDHE_PSK_WITH_AES_128_CBC_SHA256},
		Extra: psktls.PSKConfig{
			GetKey: func(identity string) ([]byte, error) {
				if identity != "test-identity" {
					t.Fatalf("unexpected identity %q", identity)
				}
				return psk, nil
			},
		},
	})
	serverErr := make(chan error, 1)
	go func() {
		serverErr <- server.Handshake()
	}()

	client := psktls.Client(clientConn, &psktls.Config{
		MinVersion:         psktls.VersionTLS12,
		MaxVersion:         psktls.VersionTLS12,
		InsecureSkipVerify: true,
		CipherSuites:       []uint16{psktls.TLS_ECDHE_PSK_WITH_AES_128_CBC_SHA256},
		Extra: psktls.PSKConfig{
			GetIdentity: func() string {
				return "test-identity"
			},
			GetKey: func(_ string) ([]byte, error) {
				return psk, nil
			},
		},
	})
	if err := client.Handshake(); err != nil {
		t.Fatalf("client handshake failed: %v", err)
	}
	if err := <-serverErr; err != nil {
		t.Fatalf("server handshake failed: %v", err)
	}
}

func TestPSKConfigCloneKeepsExtra(t *testing.T) {
	cfg := &psktls.Config{
		Extra: psktls.PSKConfig{
			GetIdentity: func() string { return "identity" },
			GetKey:      func(string) ([]byte, error) { return []byte("0123456789abcdef"), nil },
		},
	}
	cloned := cfg.Clone()
	if _, ok := cloned.Extra.(psktls.PSKConfig); !ok {
		t.Fatalf("clone dropped PSKConfig extra: %#v", cloned.Extra)
	}
}

func testPSKServerCertificate(t *testing.T) psktls.Certificate {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			CommonName: "zabbix.example.test",
		},
		NotBefore: time.Now().Add(-time.Hour),
		NotAfter:  time.Now().Add(time.Hour),
		KeyUsage:  x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
	}
	der, err := x509.CreateCertificate(rand.Reader, &template, &template, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})
	cert, err := psktls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		t.Fatal(err)
	}
	return cert
}
