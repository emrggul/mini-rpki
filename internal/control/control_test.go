package control

import (
	"bytes"
	"encoding/json"
	"github.com/minirpki/mini-rpki/internal/rtr"
	"github.com/minirpki/mini-rpki/internal/vrp"
	"net/http/httptest"
	"net/netip"
	"testing"
)

func TestControl(t *testing.T) {
	s, _ := vrp.Open("", nil)
	api := API{s, &rtr.Server{Store: s}}
	r := vrp.Record{Prefix: netip.MustParsePrefix("192.0.2.0/24"), ASN: 65001, MaxLen: 24}
	for _, action := range []string{"add", "list", "status", "remove", "clear"} {
		b, _ := json.Marshal(Request{Action: action, Record: r})
		w := httptest.NewRecorder()
		api.ServeHTTP(w, httptest.NewRequest("POST", "/control", bytes.NewReader(b)))
		if w.Code != 200 {
			t.Fatalf("%s: %d %s", action, w.Code, w.Body.String())
		}
	}
}
