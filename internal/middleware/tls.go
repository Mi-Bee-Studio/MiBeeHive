package middleware

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"log/slog"
	"math/big"
	"net"
	"os"
	"time"
)

// detectInterfaceIPs enumerates all non-loopback, up interfaces and returns their IPv4 addresses.
func detectInterfaceIPs() []net.IP {
	interfaces, err := net.Interfaces()
	if err != nil {
		slog.Warn("failed to enumerate network interfaces, falling back to loopback only", "error", err)
		return nil
	}

	var ips []net.IP
	for _, iface := range interfaces {
		if iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		if iface.Flags&net.FlagUp == 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			slog.Debug("failed to get addresses for interface", "interface", iface.Name, "error", err)
			continue
		}
		for _, addr := range addrs {
			ipNet, ok := addr.(*net.IPNet)
			if !ok {
				continue
			}
			if ip := ipNet.IP.To4(); ip != nil {
				ips = append(ips, ip)
			}
		}
	}
	return ips
}

// EnsureTLSCert generates a self-signed ECDSA P256 certificate and key
// if the files do not already exist.
//
// cfgIPs and cfgDNS allow overriding auto-detected IP addresses and DNS names.
// If cfgIPs is empty, IPs are auto-detected from network interfaces.
// If cfgDNS is empty, ["localhost"] is used as the default DNS name.
// 127.0.0.1 is always included in the IP SANs.
func EnsureTLSCert(certPath, keyPath string, cfgIPs, cfgDNS []string) error {
	if _, err := os.Stat(certPath); err == nil {
		if _, err := os.Stat(keyPath); err == nil {
			slog.Info("TLS certificate and key already exist", "cert", certPath, "key", keyPath)
			return nil
		}
	}

	slog.Info("generating self-signed TLS certificate", "cert", certPath, "key", keyPath)

	privKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return fmt.Errorf("generating ECDSA private key: %w", err)
	}

	serialNumber, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return fmt.Errorf("generating serial number: %w", err)
	}

	// Determine IP SANs: use config-provided IPs or auto-detect.
	var ips []net.IP
	if len(cfgIPs) > 0 {
		for _, s := range cfgIPs {
			if ip := net.ParseIP(s); ip != nil {
				ips = append(ips, ip)
			}
		}
	} else {
		ips = detectInterfaceIPs()
	}
	// Always ensure 127.0.0.1 is present.
	hasLoopback := false
	for _, ip := range ips {
		if ip.Equal(net.IPv4(127, 0, 0, 1)) {
			hasLoopback = true
			break
		}
	}
	if !hasLoopback {
		ips = append(ips, net.ParseIP("127.0.0.1"))
	}

	// Determine DNS names: use config-provided or default.
	dnsNames := cfgDNS
	if len(dnsNames) == 0 {
		dnsNames = []string{"localhost"}
	}

	template := &x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			CommonName: "MiBeeHive",
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(365 * 24 * time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		IPAddresses:           ips,
		DNSNames:              dnsNames,
		BasicConstraintsValid: true,
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &privKey.PublicKey, privKey)
	if err != nil {
		return fmt.Errorf("creating certificate: %w", err)
	}

	certFile, err := os.Create(certPath)
	if err != nil {
		return fmt.Errorf("creating cert file: %w", err)
	}
	defer certFile.Close()

	if err := pem.Encode(certFile, &pem.Block{Type: "CERTIFICATE", Bytes: certDER}); err != nil {
		return fmt.Errorf("writing cert PEM: %w", err)
	}

	keyFile, err := os.Create(keyPath)
	if err != nil {
		return fmt.Errorf("creating key file: %w", err)
	}
	defer keyFile.Close()

	keyDER, err := x509.MarshalECPrivateKey(privKey)
	if err != nil {
		return fmt.Errorf("marshaling EC private key: %w", err)
	}

	if err := pem.Encode(keyFile, &pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER}); err != nil {
		return fmt.Errorf("writing key PEM: %w", err)
	}

	slog.Info("self-signed TLS certificate generated successfully",
		"cert", certPath,
		"key", keyPath,
		"valid_until", template.NotAfter.Format(time.RFC3339),
	)

	return nil
}

// LoadTLSConfig loads a TLS certificate and key from the given paths.
func LoadTLSConfig(certPath, keyPath string) (*tls.Config, error) {
	cert, err := tls.LoadX509KeyPair(certPath, keyPath)
	if err != nil {
		return nil, fmt.Errorf("loading TLS key pair: %w", err)
	}
	return &tls.Config{
		Certificates: []tls.Certificate{cert},
	}, nil
}
