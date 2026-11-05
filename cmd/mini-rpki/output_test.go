package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/minirpki/mini-rpki/internal/control"
	"github.com/minirpki/mini-rpki/internal/vrp"
)

func TestReadableOutput(t *testing.T) {
	cases := []struct {
		action string
		data   any
		want   []string
	}{
		{"interface", control.AddressInfo{Listener: "10.82.0.1:8282", Endpoints: []control.Endpoint{{IP: "10.82.0.1", Port: "8282", Interface: "eth1"}}}, []string{"10.82.0.1", "8282", "eth1", "routing or port forwarding"}},
		{"list", vrp.DemoRecords(), []string{"PREFIX", "MAXLEN", "ORIGIN", "192.0.2.0/24", "2001:db8:100::/48", "AS65001"}},
		{"list", nil, []string{"Records served: 0", "No records configured."}},
		{"status", control.Status{Session: 42, Serial: 7, Records: 2, Refresh: 5, Retry: 10, Expire: 600}, []string{"Session ID: 42", "Serial: 7", "refresh 5s", "No routers connected."}},
		{"add", map[string]any{"serial": 2, "record": vrp.DemoRecords()[0], "vrps": vrp.DemoRecords()}, []string{"Saved: 192.0.2.0/24", "maxlen 24", "AS65001", "Cache serial: 2"}},
		{"remove", map[string]any{"serial": 3, "record": vrp.DemoRecords()[0], "vrps": []vrp.Record{}}, []string{"Removed: 192.0.2.0/24", "Cache serial: 3"}},
		{"clear", map[string]any{"serial": 4, "vrps": []vrp.Record{}}, []string{"All records cleared.", "Cache serial: 4"}},
	}
	for _, tc := range cases {
		t.Run(tc.action, func(t *testing.T) {
			data, err := json.Marshal(tc.data)
			if err != nil {
				t.Fatal(err)
			}
			var out bytes.Buffer
			if err := renderResponse(&out, tc.action, data, false); err != nil {
				t.Fatal(err)
			}
			for _, want := range tc.want {
				if !strings.Contains(out.String(), want) {
					t.Fatalf("missing %q in %s", want, out.String())
				}
			}
		})
	}
}
func TestJSONOutput(t *testing.T) {
	data := []byte(`[{"prefix":"192.0.2.0/24","maxLength":24,"asn":65001}]`)
	var out bytes.Buffer
	if err := renderResponse(&out, "list", data, true); err != nil {
		t.Fatal(err)
	}
	var before, after any
	json.Unmarshal(data, &before)
	if err := json.Unmarshal(out.Bytes(), &after); err != nil {
		t.Fatal(err)
	}
	a, _ := json.Marshal(before)
	b, _ := json.Marshal(after)
	if !bytes.Equal(a, b) {
		t.Fatal("JSON payload changed")
	}
}
func TestBadResponse(t *testing.T) {
	for _, action := range []string{"list", "status", "add", "remove", "clear", "interface"} {
		if err := renderResponse(new(bytes.Buffer), action, []byte("bad"), false); err == nil {
			t.Fatal(action)
		}
	}
}
