package rtr

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net"
	"net/netip"
	"os"
)

// TLSOptions identifies the credentials needed for RFC 8210 TLS transport.
// The cache certificate identifies this server; ClientCAFile identifies the
// CA that issued router client certificates.
type TLSOptions struct {
	CertFile     string
	KeyFile      string
	ClientCAFile string
}

// ListenTLS creates a mutual-TLS RTR listener.  TLS itself verifies that the
// router certificate chains to ClientCAFile.  The per-connection callback also
// enforces RFC 8210's requirement that a router certificate contain the IP
// address from which that router actually connected.
func ListenTLS(addr string, options TLSOptions) (net.Listener, error) {
	if options.CertFile == "" || options.KeyFile == "" || options.ClientCAFile == "" {
		return nil, fmt.Errorf("TLS requires certificate, key, and client CA files")
	}
	certificate, err := tls.LoadX509KeyPair(options.CertFile, options.KeyFile)
	if err != nil {
		return nil, fmt.Errorf("load TLS server certificate: %w", err)
	}
	caPEM, err := os.ReadFile(options.ClientCAFile)
	if err != nil {
		return nil, fmt.Errorf("read TLS client CA: %w", err)
	}
	clientCAs := x509.NewCertPool()
	if !clientCAs.AppendCertsFromPEM(caPEM) {
		return nil, fmt.Errorf("TLS client CA file contains no certificate")
	}
	config := &tls.Config{
		Certificates: []tls.Certificate{certificate},
		ClientAuth:   tls.RequireAndVerifyClientCert,
		ClientCAs:    clientCAs,
		MinVersion:   tls.VersionTLS12,
	}
	// VerifyConnection has no net.Conn argument.  GetConfigForClient gives us
	// that connection, so each handshake receives a clone that closes over its
	// observed TCP peer address.  Clear the callback in the clone to avoid
	// recursive configuration selection.
	config.GetConfigForClient = func(hello *tls.ClientHelloInfo) (*tls.Config, error) {
		remote, err := remoteIP(hello.Conn.RemoteAddr())
		if err != nil {
			return nil, err
		}
		perConnection := config.Clone()
		perConnection.GetConfigForClient = nil
		perConnection.VerifyConnection = func(state tls.ConnectionState) error {
			if len(state.PeerCertificates) == 0 {
				return fmt.Errorf("TLS router did not present a certificate")
			}
			for _, ip := range state.PeerCertificates[0].IPAddresses {
				if candidate, ok := netip.AddrFromSlice(ip); ok && candidate.Unmap() == remote {
					return nil
				}
			}
			return fmt.Errorf("TLS router certificate has no IP SAN matching source %s", remote)
		}
		return perConnection, nil
	}
	listener, err := tls.Listen("tcp", addr, config)
	if err != nil {
		return nil, fmt.Errorf("listen for TLS RTR: %w", err)
	}
	return listener, nil
}

func remoteIP(addr net.Addr) (netip.Addr, error) {
	tcp, ok := addr.(*net.TCPAddr)
	if !ok {
		return netip.Addr{}, fmt.Errorf("TLS router has non-TCP source address %q", addr)
	}
	ip, ok := netip.AddrFromSlice(tcp.IP)
	if !ok {
		return netip.Addr{}, fmt.Errorf("TLS router has invalid source address %q", tcp.IP)
	}
	return ip.Unmap(), nil
}
