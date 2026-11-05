package control

import (
	"bytes"
	"context"
	"encoding/json"
	"net"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/minirpki/mini-rpki/internal/rtr"
	"github.com/minirpki/mini-rpki/internal/vrp"
)

func TestAddressUsesRunningListener(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	store, err := vrp.Open("", nil)
	if err != nil {
		t.Fatal(err)
	}
	server := &rtr.Server{Store: store}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- server.Serve(ctx, listener) }()
	defer func() {
		cancel()
		if err := <-done; err != nil {
			t.Error(err)
		}
	}()
	deadline := time.Now().Add(3 * time.Second)
	for server.ListenerAddress() == "" && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	api := API{store, server}
	response := httptest.NewRecorder()
	api.ServeHTTP(response, httptest.NewRequest("POST", "/control", bytes.NewBufferString(`{"action":"address"}`)))
	if response.Code != 200 {
		t.Fatal(response.Body.String())
	}
	var info AddressInfo
	if err := json.Unmarshal(response.Body.Bytes(), &info); err != nil {
		t.Fatal(err)
	}
	_, port, _ := net.SplitHostPort(listener.Addr().String())
	if info.Listener != listener.Addr().String() || len(info.Endpoints) != 1 || info.Endpoints[0].IP != "127.0.0.1" || info.Endpoints[0].Port != port {
		t.Fatal(info)
	}
}
func TestAddressNotReady(t *testing.T) {
	api := API{RTR: &rtr.Server{}}
	if _, err := api.address(); err == nil {
		t.Fatal("reported a listener before startup")
	}
}
func TestAddressWildcard(t *testing.T) {
	for _, listener := range []string{"0.0.0.0:8282", "[::]:8282"} {
		info, err := describeAddress(listener)
		if err != nil {
			t.Fatal(err)
		}
		for _, endpoint := range info.Endpoints {
			if endpoint.IP == "0.0.0.0" || endpoint.IP == "::" || endpoint.Port != "8282" {
				t.Fatal(endpoint)
			}
			if listener == "0.0.0.0:8282" {
				ip := net.ParseIP(endpoint.IP)
				if ip == nil || ip.To4() == nil {
					t.Fatal(endpoint)
				}
			}
		}
	}
}
