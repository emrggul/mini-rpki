package rtr

import (
	"crypto/x509"
	"encoding/pem"
	"net/netip"
	"os"
	"testing"
)

func TestCreateCredentials(t *testing.T) {
	directory := t.TempDir() + "/tls"
	files, err := CreateCredentials(CredentialOptions{Directory: directory, ServerDNS: "minirpki.lab", RouterIP: netip.MustParseAddr("192.0.2.1")})
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{files.CA, files.ServerCert, files.ServerKey, files.RouterCert, files.RouterKey} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("missing %s: %v", path, err)
		}
	}
	serverPEM, err := os.ReadFile(files.ServerCert)
	if err != nil {
		t.Fatal(err)
	}
	server, err := parsePEMCertificate(serverPEM)
	if err != nil || !contains(server.DNSNames, "minirpki.lab") || len(server.ExtKeyUsage) != 1 || server.ExtKeyUsage[0] != x509.ExtKeyUsageServerAuth {
		t.Fatalf("unexpected server certificate: %v %#v", err, server)
	}
	routerPEM, err := os.ReadFile(files.RouterCert)
	if err != nil {
		t.Fatal(err)
	}
	router, err := parsePEMCertificate(routerPEM)
	if err != nil || len(router.IPAddresses) != 1 || !router.IPAddresses[0].Equal(netip.MustParseAddr("192.0.2.1").AsSlice()) || len(router.ExtKeyUsage) != 1 || router.ExtKeyUsage[0] != x509.ExtKeyUsageClientAuth {
		t.Fatalf("unexpected router certificate: %v %#v", err, router)
	}
	if _, err := CreateCredentials(CredentialOptions{Directory: directory, ServerDNS: "minirpki.lab", RouterIP: netip.MustParseAddr("192.0.2.1")}); err == nil {
		t.Fatal("overwrote credentials without force")
	}
}

func parsePEMCertificate(data []byte) (*x509.Certificate, error) {
	block, _ := pem.Decode(data)
	return x509.ParseCertificate(block.Bytes)
}

func contains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
