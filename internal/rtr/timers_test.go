package rtr

import (
	"context"
	"net"
	"testing"
)

func TestCustomRefresh(t *testing.T) {
	_, addr := startWithRefresh(t, 1)
	c := connect(t, addr)
	if err := Write(c, New(ResetQuery, 0)); err != nil {
		t.Fatal(err)
	}
	receive(t, c, CacheResponse)
	receive(t, c, IPv4)
	receive(t, c, IPv6)
	end := receive(t, c, EndOfData)
	if be.Uint32(end.Body[4:]) != 1 || be.Uint32(end.Body[8:]) != 10 || be.Uint32(end.Body[12:]) != 600 {
		t.Fatal(end)
	}
}
func TestInvalidRefresh(t *testing.T) {
	for _, refresh := range []uint32{600, 86400, 4294967295} {
		listener, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		server := Server{Refresh: refresh}
		err = server.Serve(context.Background(), listener)
		listener.Close()
		if err == nil {
			t.Fatal("accepted invalid refresh", refresh)
		}
	}
}
