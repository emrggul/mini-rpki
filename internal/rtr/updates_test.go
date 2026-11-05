package rtr

import (
	"github.com/minirpki/mini-rpki/internal/vrp"
	"net/netip"
	"testing"
	"time"
)

func TestNotificationRateLimit(t *testing.T) {
	now := time.Unix(1000, 0)
	if notificationDue(false, 2, 1, time.Time{}, now) {
		t.Fatal("notification before negotiation")
	}
	if !notificationDue(true, 2, 1, time.Time{}, now) {
		t.Fatal("first notification")
	}
	for _, d := range []time.Duration{0, time.Second, 59 * time.Second, time.Minute - time.Nanosecond} {
		if notificationDue(true, 3, 2, now, now.Add(d)) {
			t.Fatal("notification sent too soon", d)
		}
	}
	if !notificationDue(true, 4, 2, now, now.Add(time.Minute)) {
		t.Fatal("pending update lost")
	}
	if notificationDue(true, 4, 4, now, now.Add(2*time.Minute)) {
		t.Fatal("already synchronized")
	}
}
func TestBadQueries(t *testing.T) {
	_, addr := start(t)
	for _, p := range []PDU{New(ResetQuery, 0, 1), New(SerialQuery, 0), New(IPv4, 0)} {
		c := connect(t, addr)
		Write(c, p)
		receive(t, c, ErrorReport)
	}
	c := connect(t, addr)
	Write(c, Error(0, New(ResetQuery, 0), "stop"))
	if _, e := Read(c); e == nil {
		t.Fatal("responded to error report")
	}
}
func TestLiveAnnouncement(t *testing.T) {
	s, addr := start(t)
	c := connect(t, addr)
	id, serial, _ := s.Store.State()
	r := vrp.Record{Prefix: netip.MustParsePrefix("198.51.100.0/24"), MaxLen: 24, ASN: 65002}
	if e := s.Store.Edit(r, true); e != nil {
		t.Fatal(e)
	}
	Write(c, New(SerialQuery, id, serial))
	receive(t, c, CacheResponse)
	p := receive(t, c, IPv4)
	if p.Body[0] != 1 || be.Uint32(p.Body[8:]) != 65002 {
		t.Fatal(p)
	}
	receive(t, c, EndOfData)
}
