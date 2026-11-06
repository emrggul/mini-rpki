package main

import (
	"context"
	"flag"
	"fmt"
	"github.com/minirpki/mini-rpki/internal/control"
	"github.com/minirpki/mini-rpki/internal/rtr"
	"github.com/minirpki/mini-rpki/internal/vrp"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"syscall"
	"time"
)

func env(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}
func main() {
	if err := run(os.Args[1:]); err != nil {
		log.Print(err)
		os.Exit(1)
	}
}
func run(args []string) error {
	socket := env("MINIRPKI_SOCKET", "/data/control.sock")
	jsonOutput := false
	filtered := make([]string, 0, len(args))
	for _, arg := range args {
		if arg == "--json" {
			jsonOutput = true
		} else {
			filtered = append(filtered, arg)
		}
	}
	args = filtered
	if len(args) == 0 || args[0] == "--help" || args[0] == "help" {
		fmt.Println(`MiniRPKI: laboratory RTR cache, never a production validator.
mini-rpki serve [--listen :8282] [--data /data/vrps.json] [--socket /data/control.sock] [--refresh 5] [--rtr-version 0]
                [--tls-listen :324 --tls-cert FILE --tls-key FILE --tls-client-ca FILE]
mini-rpki tls init --server-dns NAME --router-ip ADDRESS [--dir /data/tls]
mini-rpki add PREFIX --asn ASN [--maxlen LENGTH]
mini-rpki remove PREFIX --asn ASN [--maxlen LENGTH]
mini-rpki list
mini-rpki clear
mini-rpki status
mini-rpki interface
Example: mini-rpki add 192.0.2.0/24 --asn 65001
Maximum length defaults to the prefix length. Remove requires the exact record.
Output: readable tables by default; append --json for scripts.
TLS: optional RFC 8210 mutual TLS listener; it requires RTR v1.
Environment: MINIRPKI_LISTEN, MINIRPKI_DATA, MINIRPKI_SOCKET, MINIRPKI_REFRESH, MINIRPKI_RTR_VERSION,
MINIRPKI_TLS_LISTEN, MINIRPKI_TLS_CERT, MINIRPKI_TLS_KEY, MINIRPKI_TLS_CLIENT_CA`)
		return nil
	}
	if args[0] == "serve" {
		f := flag.NewFlagSet("serve", flag.ContinueOnError)
		addr := f.String("listen", env("MINIRPKI_LISTEN", ":8282"), "RTR TCP address")
		data := f.String("data", env("MINIRPKI_DATA", "/data/vrps.json"), "JSON persistence path")
		sock := f.String("socket", socket, "local Unix control socket")
		refreshFlag := f.String("refresh", env("MINIRPKI_REFRESH", "5"), "RTR refresh interval in seconds (1..599)")
		versionFlag := f.String("rtr-version", env("MINIRPKI_RTR_VERSION", "0"), "RTR protocol version: 0 (RFC 6810) or 1 (RFC 8210)")
		tlsAddr := f.String("tls-listen", env("MINIRPKI_TLS_LISTEN", ""), "optional mutual-TLS RTR TCP address (RTR v1 only)")
		tlsCert := f.String("tls-cert", env("MINIRPKI_TLS_CERT", ""), "TLS server certificate PEM file")
		tlsKey := f.String("tls-key", env("MINIRPKI_TLS_KEY", ""), "TLS server private-key PEM file")
		tlsClientCA := f.String("tls-client-ca", env("MINIRPKI_TLS_CLIENT_CA", ""), "CA PEM used to verify TLS router certificates")
		if err := f.Parse(args[1:]); err != nil {
			return err
		}
		if f.NArg() != 0 {
			return fmt.Errorf("unexpected arguments")
		}
		refresh, err := strconv.ParseUint(*refreshFlag, 10, 32)
		if err != nil || refresh < 1 || refresh >= uint64(rtr.ExpireInterval) {
			return fmt.Errorf("--refresh / MINIRPKI_REFRESH must be 1..599 seconds")
		}
		version, err := strconv.ParseUint(*versionFlag, 10, 8)
		if err != nil || version > 1 {
			return fmt.Errorf("--rtr-version / MINIRPKI_RTR_VERSION must be 0 or 1")
		}
		tlsValues := []string{*tlsAddr, *tlsCert, *tlsKey, *tlsClientCA}
		tlsConfigured := false
		for _, value := range tlsValues {
			tlsConfigured = tlsConfigured || value != ""
		}
		if tlsConfigured {
			if version != 1 {
				return fmt.Errorf("TLS RTR requires --rtr-version 1")
			}
			for _, value := range tlsValues {
				if value == "" {
					return fmt.Errorf("TLS RTR requires --tls-listen, --tls-cert, --tls-key, and --tls-client-ca")
				}
			}
		}
		s, err := vrp.Open(*data, vrp.DemoRecords())
		if err != nil {
			return err
		}
		if err = os.MkdirAll(filepath.Dir(*sock), 0700); err != nil {
			return err
		}
		cl, err := control.Listen(*sock)
		if err != nil {
			return fmt.Errorf("control socket (remove stale socket only after confirming server stopped): %w", err)
		}
		defer os.Remove(*sock)
		defer cl.Close()
		l, err := net.Listen("tcp", *addr)
		if err != nil {
			return err
		}
		defer l.Close()
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer stop()
		server := &rtr.Server{Store: s, Refresh: uint32(refresh), ProtocolVersion: uint8(version)}
		var tlsServer *rtr.Server
		var tlsListener net.Listener
		if tlsConfigured {
			tlsListener, err = rtr.ListenTLS(*tlsAddr, rtr.TLSOptions{CertFile: *tlsCert, KeyFile: *tlsKey, ClientCAFile: *tlsClientCA})
			if err != nil {
				return err
			}
			defer tlsListener.Close()
			tlsServer = &rtr.Server{Store: s, Refresh: uint32(refresh), ProtocolVersion: 1}
		}
		api := &http.Server{Handler: control.API{Store: s, RTR: server, TLS: tlsServer}, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: 10 * time.Second, IdleTimeout: 30 * time.Second, MaxHeaderBytes: 4096}
		go func() {
			if e := api.Serve(cl); e != nil && e != http.ErrServerClosed {
				log.Print(e)
				stop()
			}
		}()
		go func() {
			<-ctx.Done()
			c, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			api.Shutdown(c)
		}()
		log.Printf("LAB ONLY RTR version=%d listening=%s control=%s", version, *addr, *sock)
		if !tlsConfigured {
			return server.Serve(ctx, l)
		}
		log.Printf("LAB ONLY mutual TLS RTR version=1 listening=%s", *tlsAddr)
		errs := make(chan error, 2)
		go func() { errs <- server.Serve(ctx, l) }()
		go func() { errs <- tlsServer.Serve(ctx, tlsListener) }()
		first := <-errs
		stop()
		second := <-errs
		if first != nil {
			return first
		}
		return second
	}
	if args[0] == "tls" {
		return runTLS(args[1:])
	}
	q, err := parseRequest(args)
	if err != nil {
		return err
	}
	action := q.Action
	if action == "interface" {
		q.Action = "address"
	}
	b, err := control.Call(context.Background(), socket, q)
	if err != nil {
		return err
	}
	return renderResponse(os.Stdout, action, b, jsonOutput)
}
