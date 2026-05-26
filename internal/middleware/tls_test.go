package middleware

import (
	"crypto/x509"
	"encoding/pem"
	"net"
	"os"
	"path/filepath"
	"testing"
)

func TestEnsureTLSCert(t *testing.T) {
	tmpDir := t.TempDir()
	certPath := filepath.Join(tmpDir, "server.crt")
	keyPath := filepath.Join(tmpDir, "server.key")

	// Generate cert.
	if err := EnsureTLSCert(certPath, keyPath, nil, nil); err != nil {
		t.Fatalf("EnsureTLSCert failed: %v", err)
	}

	// Verify files exist.
	if _, err := os.Stat(certPath); os.IsNotExist(err) {
		t.Fatal("cert file not created")
	}
	if _, err := os.Stat(keyPath); os.IsNotExist(err) {
		t.Fatal("key file not created")
	}

	// Parse and verify cert.
	certPEM, err := os.ReadFile(certPath)
	if err != nil {
		t.Fatalf("reading cert file: %v", err)
	}

	block, _ := pem.Decode(certPEM)
	if block == nil {
		t.Fatal("failed to decode PEM block")
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		t.Fatalf("parsing certificate: %v", err)
	}

	// Verify subject.
	if cert.Subject.CommonName != "MiBeeHive" {
		t.Errorf("expected CN=MiBeeHive, got %s", cert.Subject.CommonName)
	}

	// Verify SANs.
	sanIPs := map[string]bool{}
	for _, ip := range cert.IPAddresses {
		sanIPs[ip.String()] = true
	}
	for _, ip := range []string{"127.0.0.1"} {
		if !sanIPs[ip] {
			t.Errorf("SAN missing IP: %s", ip)
		}
	}

	sanDNS := map[string]bool{}
	for _, dns := range cert.DNSNames {
		sanDNS[dns] = true
	}
	if !sanDNS["localhost"] {
		t.Error("SAN missing DNS: localhost")
	}

	// Verify key usages.
	expectedKeyUsage := x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment
	if cert.KeyUsage != expectedKeyUsage {
		t.Errorf("expected key usage %d, got %d", expectedKeyUsage, cert.KeyUsage)
	}

	// Verify ext key usages.
	if len(cert.ExtKeyUsage) != 1 || cert.ExtKeyUsage[0] != x509.ExtKeyUsageServerAuth {
		t.Errorf("expected server auth ext key usage, got %v", cert.ExtKeyUsage)
	}
}

func TestEnsureTLSCert_Idempotent(t *testing.T) {
	tmpDir := t.TempDir()
	certPath := filepath.Join(tmpDir, "server.crt")
	keyPath := filepath.Join(tmpDir, "server.key")

	// First call generates.
	if err := EnsureTLSCert(certPath, keyPath, nil, nil); err != nil {
		t.Fatalf("first call failed: %v", err)
	}

	// Read original cert for comparison.
	original, err := os.ReadFile(certPath)
	if err != nil {
		t.Fatalf("reading original cert: %v", err)
	}

	// Second call should skip generation.
	if err := EnsureTLSCert(certPath, keyPath, nil, nil); err != nil {
		t.Fatalf("second call failed: %v", err)
	}

	// Cert should be unchanged.
	current, err := os.ReadFile(certPath)
	if err != nil {
		t.Fatalf("reading current cert: %v", err)
	}
	if string(original) != string(current) {
		t.Error("cert was regenerated on second call, expected idempotent")
	}
}

func TestLoadTLSConfig(t *testing.T) {
	tmpDir := t.TempDir()
	certPath := filepath.Join(tmpDir, "server.crt")
	keyPath := filepath.Join(tmpDir, "server.key")

	if err := EnsureTLSCert(certPath, keyPath, nil, nil); err != nil {
		t.Fatalf("EnsureTLSCert failed: %v", err)
	}

	tlsCfg, err := LoadTLSConfig(certPath, keyPath)
	if err != nil {
		t.Fatalf("LoadTLSConfig failed: %v", err)
	}
	if len(tlsCfg.Certificates) != 1 {
		t.Errorf("expected 1 certificate, got %d", len(tlsCfg.Certificates))
	}
}

func TestLoadTLSConfig_MissingFiles(t *testing.T) {
	_, err := LoadTLSConfig("/nonexistent/cert.pem", "/nonexistent/key.pem")
	if err == nil {
		t.Fatal("expected error for missing files")
	}
}

func TestEnsureTLSCert_SANLoopback(t *testing.T) {
	tmpDir := t.TempDir()
	certPath := filepath.Join(tmpDir, "server.crt")
	keyPath := filepath.Join(tmpDir, "server.key")

	if err := EnsureTLSCert(certPath, keyPath, nil, nil); err != nil {
		t.Fatalf("EnsureTLSCert failed: %v", err)
	}

	certPEM, _ := os.ReadFile(certPath)
	block, _ := pem.Decode(certPEM)
	cert, _ := x509.ParseCertificate(block.Bytes)

	// Verify loopback IPv6 is NOT included (we only put IPv4 SANs).
	for _, ip := range cert.IPAddresses {
		if ip.Equal(net.IPv6loopback) {
			t.Error("unexpected IPv6 loopback in SANs")
		}
	}
}

func TestEnsureTLSCert_WithConfigIPs(t *testing.T) {
	tmpDir := t.TempDir()
	certPath := filepath.Join(tmpDir, "server.crt")
	keyPath := filepath.Join(tmpDir, "server.key")

	// Generate cert with specific IPs and DNS names (simulating config override).
	cfgIPs := []string{"10.0.0.5", "192.168.1.100"}
	cfgDNS := []string{"mibeehive.local", "nas.local"}
	if err := EnsureTLSCert(certPath, keyPath, cfgIPs, cfgDNS); err != nil {
		t.Fatalf("EnsureTLSCert failed: %v", err)
	}

	certPEM, _ := os.ReadFile(certPath)
	block, _ := pem.Decode(certPEM)
	cert, _ := x509.ParseCertificate(block.Bytes)

	// Verify config IPs are present.
	sanIPs := map[string]bool{}
	for _, ip := range cert.IPAddresses {
		sanIPs[ip.String()] = true
	}
	if !sanIPs["10.0.0.5"] {
		t.Error("SAN missing config IP: 10.0.0.5")
	}
	if !sanIPs["192.168.1.100"] {
		t.Error("SAN missing config IP: 192.168.1.100")
	}
	if !sanIPs["127.0.0.1"] {
		t.Error("SAN missing loopback IP: 127.0.0.1 (should always be added)")
	}

	// Verify config DNS names are present (instead of default "localhost").
	sanDNS := map[string]bool{}
	for _, dns := range cert.DNSNames {
		sanDNS[dns] = true
	}
	if !sanDNS["mibeehive.local"] {
		t.Error("SAN missing DNS: mibeehive.local")
	}
	if !sanDNS["nas.local"] {
		t.Error("SAN missing DNS: nas.local")
	}
	// "localhost" should NOT be present since custom DNS names were provided.
	if sanDNS["localhost"] {
		t.Error("unexpected DNS localhost present when custom DNS names were configured")
	}
}

func TestDetectInterfaceIPs(t *testing.T) {
	ips := detectInterfaceIPs()
	// Verify no loopback IPs are returned (function filters loopback interfaces).
	for _, ip := range ips {
		if ip.IsLoopback() {
			t.Errorf("detectInterfaceIPs returned loopback IP: %s", ip.String())
		}
	}
	// Verify all returned IPs are IPv4.
	for _, ip := range ips {
		if ip.To4() == nil {
			t.Errorf("detectInterfaceIPs returned non-IPv4 IP: %s", ip.String())
		}
	}
}
