package control

import (
	"context"
	"encoding/json"
	"github.com/minirpki/mini-rpki/internal/rtr"
	"github.com/minirpki/mini-rpki/internal/vrp"
	"net/http"
	"path/filepath"
	"testing"
	"time"
)

func TestUnixControl(t *testing.T) {
	path := filepath.Join(t.TempDir(), "c.sock")
	l, err := Listen(path)
	if err != nil {
		t.Fatal(err)
	}
	s, err := vrp.Open("", nil)
	if err != nil {
		t.Fatal(err)
	}
	server := &http.Server{Handler: API{Store: s, RTR: &rtr.Server{Store: s}}, ReadHeaderTimeout: time.Second}
	done := make(chan error, 1)
	go func() { done <- server.Serve(l) }()
	defer func() { server.Close(); <-done }()
	b, err := Call(context.Background(), path, Request{Action: "add", Record: vrp.DemoRecords()[0]})
	if err != nil {
		t.Fatal(err)
	}
	if !json.Valid(b) {
		t.Fatal(string(b))
	}
	b, err = Call(context.Background(), path, Request{Action: "list"})
	if err != nil {
		t.Fatal(err)
	}
	var records []vrp.Record
	if err = json.Unmarshal(b, &records); err != nil || len(records) != 1 {
		t.Fatal(string(b), err)
	}
}
