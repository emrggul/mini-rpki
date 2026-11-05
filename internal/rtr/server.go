package rtr

import (
	"context"
	"encoding/binary"
	"fmt"
	"github.com/minirpki/mini-rpki/internal/vrp"
	"log"
	"net"
	"sync"
	"time"
)

type Client struct {
	Address string    `json:"address"`
	Since   time.Time `json:"since"`
}

// notificationDue leaves pending serials untouched until the one-minute boundary.
func notificationDue(negotiated bool, serial, seen uint32, last, now time.Time) bool {
	return negotiated && serial != seen && (last.IsZero() || now.Sub(last) >= time.Minute)
}

const (
	DefaultRefresh uint32 = 5
	RetryInterval  uint32 = 10
	ExpireInterval uint32 = 600
)

type Server struct {
	// ProtocolVersion is the configured RTR version. Zero is intentional: it
	// makes RFC 6810 / RTR v0 the safe zero-value and command-line default.
	ProtocolVersion uint8
	Refresh         uint32
	Store           *vrp.Store
	mu              sync.Mutex
	clients         map[net.Conn]Client
	wg              sync.WaitGroup
	listenerAddress string
}

func (s *Server) ListenerAddress() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.listenerAddress
}
func (s *Server) RefreshInterval() uint32 {
	if s.Refresh == 0 {
		return DefaultRefresh
	}
	return s.Refresh
}
func (s *Server) RTRVersion() uint8 { return s.ProtocolVersion }
func (s *Server) Clients() []Client {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := []Client{}
	for _, c := range s.clients {
		out = append(out, c)
	}
	return out
}
func (s *Server) Serve(ctx context.Context, l net.Listener) error {
	if s.RTRVersion() > 1 {
		return fmt.Errorf("RTR protocol version must be 0 or 1")
	}
	if s.RefreshInterval() >= ExpireInterval {
		return fmt.Errorf("refresh interval must be 1..599 seconds (expire is 600)")
	}
	s.mu.Lock()
	s.clients = make(map[net.Conn]Client)
	s.listenerAddress = l.Addr().String()
	s.mu.Unlock()
	defer func() { s.mu.Lock(); s.listenerAddress = ""; s.mu.Unlock() }()
	done := make(chan struct{})
	defer close(done)
	go func() {
		select {
		case <-ctx.Done():
			l.Close()
			s.mu.Lock()
			for c := range s.clients {
				c.Close()
			}
			s.mu.Unlock()
		case <-done:
		}
	}()
	defer s.wg.Wait()
	for {
		c, err := l.Accept()
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return err
		}
		s.mu.Lock()
		if ctx.Err() != nil {
			c.Close()
			s.mu.Unlock()
			return nil
		}
		s.clients[c] = Client{c.RemoteAddr().String(), time.Now()}
		s.mu.Unlock()
		s.wg.Add(1)
		go func() { defer s.wg.Done(); s.handle(ctx, c) }()
	}
}
func (s *Server) handle(ctx context.Context, c net.Conn) {
	defer func() { c.Close(); s.mu.Lock(); delete(s.clients, c); s.mu.Unlock() }()
	log.Printf("RTR connected remote=%s", c.RemoteAddr())
	// One writer serializes entire responses and notifications, preventing PDU interleaving.
	var writeMu sync.Mutex
	negotiated := false
	var seen uint32
	var lastNotify time.Time
	closed := make(chan struct{})
	defer close(closed)
	go func() {
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-closed:
				return
			case <-ticker.C:
				writeMu.Lock()
				session, serial, _ := s.Store.State()
				if notificationDue(negotiated, serial, seen, lastNotify, time.Now()) {
					c.SetWriteDeadline(time.Now().Add(10 * time.Second))
					if Write(c, NewVersion(s.RTRVersion(), Notify, session, serial)) != nil {
						c.Close()
						writeMu.Unlock()
						return
					}
					lastNotify = time.Now()
					seen = serial
					log.Printf("RTR Serial Notify remote=%s serial=%d", c.RemoteAddr(), serial)
				}
				writeMu.Unlock()
			}
		}
	}()
	for {
		c.SetReadDeadline(time.Now().Add(2 * time.Hour))
		p, err := Read(c)
		if err != nil {
			if p.Version != 0 || p.Type != 0 {
				if p.Type != ErrorReport {
					writeMu.Lock()
					c.SetWriteDeadline(time.Now().Add(10 * time.Second))
					_ = Write(c, ErrorVersion(s.RTRVersion(), 0, p, err.Error()))
					writeMu.Unlock()
				}
			}
			return
		}
		writeMu.Lock()
		c.SetWriteDeadline(time.Now().Add(10 * time.Second))
		if p.Type == ErrorReport {
			writeMu.Unlock()
			return
		}
		if p.Version > 1 {
			_ = Write(c, ErrorVersion(s.RTRVersion(), 4, p, "unsupported RTR protocol version"))
			writeMu.Unlock()
			return
		}
		// A cache configured for v1 is v1-only.  RFC 8210 permits it to
		// decline a v0 query with Unsupported Protocol Version.  Conversely,
		// a configured v0 cache answers an initial v1 query in v0 so the router
		// can perform the defined downgrade.
		if !negotiated && s.RTRVersion() == 1 && p.Version != 1 {
			_ = Write(c, ErrorVersion(1, 4, p, "RTR version 1 is configured"))
			writeMu.Unlock()
			return
		}
		if negotiated && p.Version != s.RTRVersion() {
			code := uint16(0)
			if s.RTRVersion() == 1 {
				code = 8 // RFC 8210 Unexpected Protocol Version.
			}
			_ = Write(c, ErrorVersion(s.RTRVersion(), code, p, "unexpected RTR protocol version"))
			writeMu.Unlock()
			return
		}
		if p.Type != ResetQuery && p.Type != SerialQuery {
			_ = Write(c, ErrorVersion(s.RTRVersion(), 5, p, "expected Reset Query or Serial Query"))
			writeMu.Unlock()
			return
		}
		if (p.Type == ResetQuery && len(p.Body) != 0) || (p.Type == SerialQuery && len(p.Body) != 4) {
			_ = Write(c, ErrorVersion(s.RTRVersion(), 0, p, "invalid query length"))
			writeMu.Unlock()
			return
		}
		if !negotiated {
			log.Printf("RTR version negotiation remote=%s requested=%d selected=%d", c.RemoteAddr(), p.Version, s.RTRVersion())
		}
		negotiated = true
		session, serial, records := s.Store.State()
		var withdrawals []vrp.Record
		if p.Type == SerialQuery {
			if p.Field != session {
				_ = Write(c, ErrorVersion(s.RTRVersion(), 0, p, "session ID mismatch"))
				writeMu.Unlock()
				return
			}
			var ok bool
			session, serial, records, withdrawals, ok = s.Store.Delta(p.Field, binary.BigEndian.Uint32(p.Body))
			if !ok {
				err = Write(c, NewVersion(s.RTRVersion(), CacheReset, 0))
				writeMu.Unlock()
				if err != nil {
					return
				}
				continue
			}
		}
		log.Printf("RTR query remote=%s type=%d serial=%d announcements=%d withdrawals=%d", c.RemoteAddr(), p.Type, serial, len(records), len(withdrawals))
		err = Write(c, NewVersion(s.RTRVersion(), CacheResponse, session))
		for _, r := range withdrawals {
			if err == nil {
				err = Write(c, PrefixVersion(s.RTRVersion(), r, false))
			}
		}
		for _, r := range records {
			if err == nil {
				err = Write(c, PrefixVersion(s.RTRVersion(), r, true))
			}
		}
		if err == nil {
			err = Write(c, EndOfDataVersion(s.RTRVersion(), session, serial, s.RefreshInterval(), RetryInterval, ExpireInterval))
		}
		if err == nil {
			seen = serial
			log.Printf("RTR End of Data remote=%s serial=%d", c.RemoteAddr(), serial)
		}
		writeMu.Unlock()
		if err != nil {
			return
		}
	}
}
