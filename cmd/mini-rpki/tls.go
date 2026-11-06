package main

import (
	"flag"
	"fmt"
	"net/netip"
	"os"

	"github.com/minirpki/mini-rpki/internal/rtr"
)

func runTLS(args []string) error {
	if len(args) == 0 || args[0] != "init" {
		return fmt.Errorf("usage: mini-rpki tls init --server-dns NAME --router-ip ADDRESS [--dir /data/tls]")
	}
	flags := flag.NewFlagSet("tls init", flag.ContinueOnError)
	directory := flags.String("dir", "/data/tls", "directory for TLS certificates and private keys")
	serverDNS := flags.String("server-dns", "", "DNS name configured by the router for the MiniRPKI server")
	routerIP := flags.String("router-ip", "", "router source IP address; placed in its client certificate IP SAN")
	force := flags.Bool("force", false, "replace existing TLS files after backing them up")
	if err := flags.Parse(args[1:]); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("unexpected arguments")
	}
	ip, err := netip.ParseAddr(*routerIP)
	if err != nil {
		return fmt.Errorf("invalid or missing --router-ip")
	}
	files, err := rtr.CreateCredentials(rtr.CredentialOptions{Directory: *directory, ServerDNS: *serverDNS, RouterIP: ip, Force: *force})
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(os.Stdout, "Created lab TLS credentials:\nCA certificate: %s\nMiniRPKI server certificate: %s\nMiniRPKI server key: %s\nRouter certificate: %s\nRouter key: %s\n\nCopy the CA certificate and router certificate/key to the router. Keep private keys secret. Configure MiniRPKI with --rtr-version 1, --tls-listen, --tls-cert, --tls-key, and --tls-client-ca.\n", files.CA, files.ServerCert, files.ServerKey, files.RouterCert, files.RouterKey)
	return err
}
