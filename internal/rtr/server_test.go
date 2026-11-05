package rtr

import (
	"context"
	"github.com/minirpki/mini-rpki/internal/vrp"
	"net"
	"net/netip"
	"sync"
	"testing"
	"time"
)

func start(t *testing.T) (*Server, string) { return startWithRefresh(t, 0) }
func startWithRefresh(t *testing.T, refresh uint32) (*Server, string) {
	t.Helper()
	store, e := vrp.Open("", []vrp.Record{{Prefix: netip.MustParsePrefix("192.0.2.0/24"), MaxLen: 24, ASN: 65001}, {Prefix: netip.MustParsePrefix("2001:db8::/32"), MaxLen: 48, ASN: 65001}})
	if e != nil {
		t.Fatal(e)
	}
	// Existing exchange tests cover RFC 8210. RTR v0 has a dedicated test
	// below; production's zero-value default is RTR v0.
	s := &Server{Store: store, Refresh: refresh, ProtocolVersion: 1}
	l, e := net.Listen("tcp", "127.0.0.1:0")
	if e != nil {
		t.Fatal(e)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- s.Serve(ctx, l) }()
	t.Cleanup(func() {
		cancel()
		select {
		case e := <-done:
			if e != nil {
				t.Error(e)
			}
		case <-time.After(5 * time.Second):
			t.Error("shutdown timeout")
		}
	})
	return s, l.Addr().String()
}
func connect(t *testing.T, addr string) net.Conn {
	t.Helper()
	c, e := net.Dial("tcp", addr)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { c.Close() })
	c.SetDeadline(time.Now().Add(5 * time.Second))
	return c
}
func receive(t *testing.T, c net.Conn, typ uint8) PDU {
	t.Helper()
	for {
		p, e := Read(c)
		if e != nil {
			t.Fatal(e)
		}
		if p.Type == Notify {
			continue
		}
		if p.Type != typ {
			t.Fatalf("got type %d want %d", p.Type, typ)
		}
		return p
	}
}
func TestExchange(t *testing.T) {
	s, addr := start(t)
	c := connect(t, addr)
	Write(c, New(ResetQuery, 0))
	response := receive(t, c, CacheResponse)
	receive(t, c, IPv4)
	receive(t, c, IPv6)
	end := receive(t, c, EndOfData)
	if len(end.Body) != 16 || be.Uint32(end.Body[4:]) != 5 || be.Uint32(end.Body[12:]) != 600 {
		t.Fatal(end)
	}
	serial := be.Uint32(end.Body)
	_, _, rs := s.Store.State()
	s.Store.Edit(rs[0], false)
	Write(c, New(SerialQuery, response.Field, serial))
	receive(t, c, CacheResponse)
	p := receive(t, c, IPv4)
	if p.Body[0] != 0 {
		t.Fatal("expected withdrawal")
	}
	end = receive(t, c, EndOfData)
	Write(c, New(SerialQuery, response.Field, be.Uint32(end.Body)))
	receive(t, c, CacheResponse)
	receive(t, c, EndOfData)
	Write(c, New(SerialQuery, response.Field+1, serial))
	report := receive(t, c, ErrorReport)
	if report.Field != 0 {
		t.Fatalf("got Error Report code %d, want 0", report.Field)
	}
	if _, err := Read(c); err == nil {
		t.Fatal("session stayed open after a Session ID mismatch")
	}
	c = connect(t, addr)
	Write(c, New(SerialQuery, response.Field, 99999))
	receive(t, c, CacheReset)
}
func TestVersions(t *testing.T) {
	_, addr := start(t)
	for _, v := range []byte{0, 2, 255} {
		c := connect(t, addr)
		p := New(ResetQuery, 0)
		p.Version = v
		Write(c, p)
		e := receive(t, c, ErrorReport)
		if e.Field != 4 || e.Version != 1 {
			t.Fatal(e)
		}
	}
	c := connect(t, addr)
	Write(c, New(ResetQuery, 0))
	receive(t, c, CacheResponse)
	receive(t, c, IPv4)
	receive(t, c, IPv6)
	receive(t, c, EndOfData)
	p := New(ResetQuery, 0)
	p.Version = 0
	Write(c, p)
	if receive(t, c, ErrorReport).Field != 8 {
		t.Fatal("unexpected version")
	}
}
func TestVersionZeroDowngradeAndEndOfData(t *testing.T) {
	store, err := vrp.Open("", []vrp.Record{{Prefix: netip.MustParsePrefix("192.0.2.0/24"), MaxLen: 24, ASN: 65001}})
	if err != nil {
		t.Fatal(err)
	}
	s := &Server{Store: store} // Zero value deliberately means RTR v0.
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- s.Serve(ctx, l) }()
	t.Cleanup(func() { cancel(); <-done })
	c := connect(t, l.Addr().String())
	// A v1 router's initial query is answered in v0, which is the RFC 8210
	// downgrade mechanism for a cache configured as v0-only.
	Write(c, New(ResetQuery, 0))
	response := receive(t, c, CacheResponse)
	if response.Version != 0 {
		t.Fatalf("response version=%d, want 0", response.Version)
	}
	prefix := receive(t, c, IPv4)
	if prefix.Version != 0 {
		t.Fatalf("prefix version=%d, want 0", prefix.Version)
	}
	end := receive(t, c, EndOfData)
	if end.Version != 0 || len(end.Body) != 4 {
		t.Fatalf("v0 End of Data = version %d body length %d, want version 0 length 4", end.Version, len(end.Body))
	}
}
func TestConcurrentClients(t *testing.T) {
	_, addr := start(t)
	var wg sync.WaitGroup
	errs := make(chan error, 20)
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			c, e := net.Dial("tcp", addr)
			if e != nil {
				errs <- e
				return
			}
			defer c.Close()
			c.SetDeadline(time.Now().Add(5 * time.Second))
			if e = Write(c, New(ResetQuery, 0)); e != nil {
				errs <- e
				return
			}
			for _, typ := range []uint8{CacheResponse, IPv4, IPv6, EndOfData} {
				p, e := Read(c)
				if e != nil {
					errs <- e
					return
				}
				if p.Type != typ {
					errs <- context.Canceled
					return
				}
			}
		}()
	}
	wg.Wait()
	close(errs)
	for e := range errs {
		t.Error(e)
	}
}
func TestRestart(t *testing.T) {
	old, _ := vrp.Open("", nil)
	id, serial, _ := old.State()
	s, addr := start(t)
	current, _, _ := s.Store.State()
	if id == current {
		id++
	}
	c := connect(t, addr)
	Write(c, New(SerialQuery, id, serial))
	report := receive(t, c, ErrorReport)
	if report.Field != 0 {
		t.Fatalf("got Error Report code %d, want 0", report.Field)
	}
	if _, err := Read(c); err == nil {
		t.Fatal("session stayed open after a Session ID mismatch")
	}
}
