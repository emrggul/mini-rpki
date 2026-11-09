package rtr

import (
	"crypto/rsa"
	"crypto/tls"
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
		info, err := os.Stat(path)
		if err != nil {
			t.Fatalf("missing %s: %v", path, err)
		}
		if info.Mode().Perm() != 0644 {
			t.Fatalf("%s permissions=%#o, want 0644", path, info.Mode().Perm())
		}
	}
	info, err := os.Stat(directory)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0755 {
		t.Fatalf("credential directory permissions=%#o, want 0755", info.Mode().Perm())
	}
	serverPEM, err := os.ReadFile(files.ServerCert)
	if err != nil {
		t.Fatal(err)
	}
	server, err := parsePEMCertificate(serverPEM)
	if err != nil || !contains(server.DNSNames, "minirpki.lab") || len(server.ExtKeyUsage) != 1 || server.ExtKeyUsage[0] != x509.ExtKeyUsageServerAuth || server.KeyUsage&(x509.KeyUsageDigitalSignature|x509.KeyUsageKeyEncipherment) != x509.KeyUsageDigitalSignature|x509.KeyUsageKeyEncipherment {
		t.Fatalf("unexpected server certificate: %v %#v", err, server)
	}
	if _, ok := server.PublicKey.(*rsa.PublicKey); !ok {
		t.Fatalf("server certificate key is %T, want RSA", server.PublicKey)
	}
	routerPEM, err := os.ReadFile(files.RouterCert)
	if err != nil {
		t.Fatal(err)
	}
	router, err := parsePEMCertificate(routerPEM)
	if err != nil || len(router.IPAddresses) != 1 || !router.IPAddresses[0].Equal(netip.MustParseAddr("192.0.2.1").AsSlice()) || len(router.ExtKeyUsage) != 1 || router.ExtKeyUsage[0] != x509.ExtKeyUsageClientAuth || router.KeyUsage&(x509.KeyUsageDigitalSignature|x509.KeyUsageKeyEncipherment) != x509.KeyUsageDigitalSignature|x509.KeyUsageKeyEncipherment {
		t.Fatalf("unexpected router certificate: %v %#v", err, router)
	}
	if _, ok := router.PublicKey.(*rsa.PublicKey); !ok {
		t.Fatalf("router certificate key is %T, want RSA", router.PublicKey)
	}
	if _, err := CreateCredentials(CredentialOptions{Directory: directory, ServerDNS: "minirpki.lab", RouterIP: netip.MustParseAddr("192.0.2.1")}); err == nil {
		t.Fatal("overwrote credentials without force")
	}
}

func TestGeneratedCredentialsWorkForMutualTLS(t *testing.T) {
	files, err := CreateCredentials(CredentialOptions{Directory: t.TempDir() + "/tls", ServerDNS: "minirpki.lab", RouterIP: netip.MustParseAddr("127.0.0.1")})
	if err != nil {
		t.Fatal(err)
	}
	listener, err := ListenTLS("127.0.0.1:0", TLSOptions{CertFile: files.ServerCert, KeyFile: files.ServerKey, ClientCAFile: files.CA})
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	clientCertificate, err := tls.LoadX509KeyPair(files.RouterCert, files.RouterKey)
	if err != nil {
		t.Fatal(err)
	}
	caPEM, err := os.ReadFile(files.CA)
	if err != nil {
		t.Fatal(err)
	}
	roots := x509.NewCertPool()
	roots.AppendCertsFromPEM(caPEM)
	accepted := make(chan error, 1)
	go func() {
		connection, err := listener.Accept()
		if err == nil {
			err = connection.(*tls.Conn).Handshake()
			connection.Close()
		}
		accepted <- err
	}()
	client, err := tls.Dial("tcp", listener.Addr().String(), &tls.Config{ServerName: "minirpki.lab", RootCAs: roots, Certificates: []tls.Certificate{clientCertificate}, MinVersion: tls.VersionTLS12})
	if err != nil {
		t.Fatal(err)
	}
	client.Close()
	if err := <-accepted; err != nil {
		t.Fatal(err)
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
