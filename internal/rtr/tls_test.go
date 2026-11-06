package rtr

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"net/netip"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/minirpki/mini-rpki/internal/vrp"
)

func writeTLSMaterial(t *testing.T, clientIPs []net.IP) (TLSOptions, tls.Certificate, *x509.CertPool) {
	t.Helper()
	directory := t.TempDir()
	caKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	caTemplate := &x509.Certificate{
		SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "MiniRPKI test CA"},
		NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour),
		IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
	}
	caDER, err := x509.CreateCertificate(rand.Reader, caTemplate, caTemplate, &caKey.PublicKey, caKey)
	if err != nil {
		t.Fatal(err)
	}
	ca, err := x509.ParseCertificate(caDER)
	if err != nil {
		t.Fatal(err)
	}
	serverCert, serverKey := issueTLSCertificate(t, ca, caKey, "minirpki.test", nil, false, 2)
	clientCert, clientKey := issueTLSCertificate(t, ca, caKey, "router.test", clientIPs, true, 3)
	serverKeyDER, err := x509.MarshalPKCS8PrivateKey(serverKey)
	if err != nil {
		t.Fatal(err)
	}
	clientKeyDER, err := x509.MarshalPKCS8PrivateKey(clientKey)
	if err != nil {
		t.Fatal(err)
	}
	serverCertPath := filepath.Join(directory, "server.pem")
	serverKeyPath := filepath.Join(directory, "server-key.pem")
	caPath := filepath.Join(directory, "ca.pem")
	writePEM(t, serverCertPath, "CERTIFICATE", serverCert)
	writePEM(t, serverKeyPath, "PRIVATE KEY", serverKeyDER)
	writePEM(t, caPath, "CERTIFICATE", caDER)
	clientPair, err := tls.X509KeyPair(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: clientCert}), pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: clientKeyDER}))
	if err != nil {
		t.Fatal(err)
	}
	roots := x509.NewCertPool()
	roots.AddCert(ca)
	return TLSOptions{CertFile: serverCertPath, KeyFile: serverKeyPath, ClientCAFile: caPath}, clientPair, roots
}

func issueTLSCertificate(t *testing.T, ca *x509.Certificate, caKey *ecdsa.PrivateKey, dns string, ips []net.IP, client bool, serial int64) ([]byte, *ecdsa.PrivateKey) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	usage := x509.ExtKeyUsageServerAuth
	if client {
		usage = x509.ExtKeyUsageClientAuth
	}
	template := &x509.Certificate{
		SerialNumber: big.NewInt(serial), Subject: pkix.Name{CommonName: dns}, DNSNames: []string{dns}, IPAddresses: ips,
		NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour),
		KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{usage},
	}
	der, err := x509.CreateCertificate(rand.Reader, template, ca, &key.PublicKey, caKey)
	if err != nil {
		t.Fatal(err)
	}
	return der, key
}

func writePEM(t *testing.T, path, kind string, der []byte) {
	t.Helper()
	if err := os.WriteFile(path, pem.EncodeToMemory(&pem.Block{Type: kind, Bytes: der}), 0600); err != nil {
		t.Fatal(err)
	}
}

func TestTLSListenerRequiresMatchingRouterIPSan(t *testing.T) {
	options, clientCertificate, roots := writeTLSMaterial(t, []net.IP{net.ParseIP("127.0.0.1")})
	listener, err := ListenTLS("127.0.0.1:0", options)
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	accepted := make(chan error, 1)
	go func() {
		connection, err := listener.Accept()
		if err == nil {
			err = connection.(*tls.Conn).Handshake()
			connection.Close()
		}
		accepted <- err
	}()
	client, err := tls.Dial("tcp", listener.Addr().String(), &tls.Config{ServerName: "minirpki.test", RootCAs: roots, Certificates: []tls.Certificate{clientCertificate}, MinVersion: tls.VersionTLS12})
	if err != nil {
		t.Fatal(err)
	}
	client.Close()
	if err := <-accepted; err != nil {
		t.Fatal(err)
	}
}

func TestTLSListenerRejectsWrongRouterIPSan(t *testing.T) {
	options, clientCertificate, roots := writeTLSMaterial(t, []net.IP{net.ParseIP("192.0.2.1")})
	listener, err := ListenTLS("127.0.0.1:0", options)
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	accepted := make(chan error, 1)
	go func() {
		connection, err := listener.Accept()
		if err == nil {
			err = connection.(*tls.Conn).Handshake()
			connection.Close()
		}
		accepted <- err
	}()
	client, err := tls.Dial("tcp", listener.Addr().String(), &tls.Config{ServerName: "minirpki.test", RootCAs: roots, Certificates: []tls.Certificate{clientCertificate}, MinVersion: tls.VersionTLS12})
	if err == nil {
		client.Close()
	}
	if err := <-accepted; err == nil {
		t.Fatal("server accepted a mismatched router IP SAN")
	}
}

func TestRTRSessionOverMutualTLS(t *testing.T) {
	options, clientCertificate, roots := writeTLSMaterial(t, []net.IP{net.ParseIP("127.0.0.1")})
	listener, err := ListenTLS("127.0.0.1:0", options)
	if err != nil {
		t.Fatal(err)
	}
	store, err := vrp.Open("", []vrp.Record{{Prefix: netip.MustParsePrefix("192.0.2.0/24"), MaxLen: 24, ASN: 65001}})
	if err != nil {
		listener.Close()
		t.Fatal(err)
	}
	server := &Server{Store: store, ProtocolVersion: 1}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- server.Serve(ctx, listener) }()
	defer func() {
		cancel()
		if err := <-done; err != nil {
			t.Error(err)
		}
	}()
	client, err := tls.Dial("tcp", listener.Addr().String(), &tls.Config{ServerName: "minirpki.test", RootCAs: roots, Certificates: []tls.Certificate{clientCertificate}, MinVersion: tls.VersionTLS12})
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	client.SetDeadline(time.Now().Add(5 * time.Second))
	if err := Write(client, New(ResetQuery, 0)); err != nil {
		t.Fatal(err)
	}
	if response := receive(t, client, CacheResponse); response.Version != 1 {
		t.Fatalf("RTR response version=%d, want 1", response.Version)
	}
	receive(t, client, IPv4)
	receive(t, client, EndOfData)
}

func TestTLSOptionsRequireAllFiles(t *testing.T) {
	if _, err := ListenTLS("127.0.0.1:0", TLSOptions{}); err == nil {
		t.Fatal("accepted incomplete TLS options")
	}
	if _, ok := netip.AddrFromSlice(net.ParseIP("127.0.0.1")); !ok {
		t.Fatal("test environment could not parse loopback")
	}
}
