package rtr

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"net/netip"
	"os"
	"path/filepath"
	"time"
)

// TLSRSABits deliberately favors broad router interoperability.  SR OS
// requires key encipherment for its TLS certificate profile, which is an RSA
// capability; 3072 bits is a modern long-lived lab key size.
const TLSRSABits = 3072

// CredentialOptions describes a deliberately small, private lab PKI: one CA,
// one cache server identity, and one router identity.
type CredentialOptions struct {
	Directory string
	ServerDNS string
	RouterIP  netip.Addr
	Force     bool
}

// CredentialFiles are the generated PEM files. MiniRPKI is a lab-only tool,
// so all files are intentionally host-readable through the /data bind mount.
type CredentialFiles struct {
	CA         string
	ServerCert string
	ServerKey  string
	RouterCert string
	RouterKey  string
}

func CreateCredentials(options CredentialOptions) (CredentialFiles, error) {
	if options.Directory == "" || options.ServerDNS == "" || !options.RouterIP.IsValid() {
		return CredentialFiles{}, fmt.Errorf("TLS credentials require directory, server DNS name, and router IP")
	}
	if options.RouterIP.Zone() != "" {
		return CredentialFiles{}, fmt.Errorf("router IP must not have an IPv6 zone")
	}
	files := CredentialFiles{
		CA: filepath.Join(options.Directory, "ca.pem"), ServerCert: filepath.Join(options.Directory, "server.pem"), ServerKey: filepath.Join(options.Directory, "server-key.pem"),
		RouterCert: filepath.Join(options.Directory, "router.pem"), RouterKey: filepath.Join(options.Directory, "router-key.pem"),
	}
	if err := os.MkdirAll(options.Directory, 0755); err != nil {
		return CredentialFiles{}, err
	}
	if err := os.Chmod(options.Directory, 0755); err != nil {
		return CredentialFiles{}, err
	}
	for _, path := range []string{files.CA, files.ServerCert, files.ServerKey, files.RouterCert, files.RouterKey} {
		if _, err := os.Stat(path); err == nil && !options.Force {
			return CredentialFiles{}, fmt.Errorf("refusing to overwrite existing TLS file %s (use --force after backing it up)", path)
		} else if err != nil && !os.IsNotExist(err) {
			return CredentialFiles{}, err
		}
	}
	caKey, err := rsa.GenerateKey(rand.Reader, TLSRSABits)
	if err != nil {
		return CredentialFiles{}, err
	}
	now := time.Now()
	caTemplate := &x509.Certificate{SerialNumber: certificateSerial(), Subject: pkix.Name{CommonName: "MiniRPKI lab CA"}, NotBefore: now.Add(-time.Minute), NotAfter: now.AddDate(5, 0, 0), IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign | x509.KeyUsageCRLSign | x509.KeyUsageDigitalSignature}
	caDER, err := x509.CreateCertificate(rand.Reader, caTemplate, caTemplate, &caKey.PublicKey, caKey)
	if err != nil {
		return CredentialFiles{}, err
	}
	ca, err := x509.ParseCertificate(caDER)
	if err != nil {
		return CredentialFiles{}, err
	}
	serverDER, serverKey, err := signedCertificate(ca, caKey, "MiniRPKI RTR server", []string{options.ServerDNS}, nil, x509.ExtKeyUsageServerAuth)
	if err != nil {
		return CredentialFiles{}, err
	}
	routerDER, routerKey, err := signedCertificate(ca, caKey, "MiniRPKI RTR router", nil, []netip.Addr{options.RouterIP.Unmap()}, x509.ExtKeyUsageClientAuth)
	if err != nil {
		return CredentialFiles{}, err
	}
	if err := writeCredentialFiles(files, caDER, serverDER, serverKey, routerDER, routerKey, options.Force); err != nil {
		return CredentialFiles{}, err
	}
	return files, nil
}

func certificateSerial() *big.Int {
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil || serial.Sign() == 0 {
		return big.NewInt(time.Now().UnixNano())
	}
	return serial
}

func signedCertificate(ca *x509.Certificate, caKey crypto.Signer, commonName string, dnsNames []string, ips []netip.Addr, usage x509.ExtKeyUsage) ([]byte, crypto.Signer, error) {
	key, err := rsa.GenerateKey(rand.Reader, TLSRSABits)
	if err != nil {
		return nil, nil, err
	}
	addressSANs := make([]net.IP, 0, len(ips))
	for _, ip := range ips {
		addressSANs = append(addressSANs, ip.AsSlice())
	}
	now := time.Now()
	template := &x509.Certificate{SerialNumber: certificateSerial(), Subject: pkix.Name{CommonName: commonName}, DNSNames: dnsNames, IPAddresses: addressSANs, NotBefore: now.Add(-time.Minute), NotAfter: now.AddDate(1, 0, 0), KeyUsage: x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment, ExtKeyUsage: []x509.ExtKeyUsage{usage}}
	der, err := x509.CreateCertificate(rand.Reader, template, ca, &key.PublicKey, caKey)
	return der, key, err
}

func writeCredentialFiles(files CredentialFiles, caDER, serverDER []byte, serverKey crypto.Signer, routerDER []byte, routerKey crypto.Signer, force bool) error {
	serverKeyDER, err := x509.MarshalPKCS8PrivateKey(serverKey)
	if err != nil {
		return err
	}
	routerKeyDER, err := x509.MarshalPKCS8PrivateKey(routerKey)
	if err != nil {
		return err
	}
	for _, item := range []struct {
		path, kind string
		der        []byte
		mode       os.FileMode
	}{
		{files.CA, "CERTIFICATE", caDER, 0644}, {files.ServerCert, "CERTIFICATE", serverDER, 0644}, {files.ServerKey, "PRIVATE KEY", serverKeyDER, 0644}, {files.RouterCert, "CERTIFICATE", routerDER, 0644}, {files.RouterKey, "PRIVATE KEY", routerKeyDER, 0644},
	} {
		flags := os.O_WRONLY | os.O_CREATE | os.O_TRUNC
		if !force {
			flags = os.O_WRONLY | os.O_CREATE | os.O_EXCL
		}
		file, err := os.OpenFile(item.path, flags, item.mode)
		if err != nil {
			return err
		}
		if err := file.Chmod(item.mode); err != nil {
			file.Close()
			return err
		}
		_, writeErr := file.Write(pem.EncodeToMemory(&pem.Block{Type: item.kind, Bytes: item.der}))
		closeErr := file.Close()
		if writeErr != nil {
			return writeErr
		}
		if closeErr != nil {
			return closeErr
		}
	}
	return nil
}
