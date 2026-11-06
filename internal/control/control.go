package control

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/minirpki/mini-rpki/internal/rtr"
	"github.com/minirpki/mini-rpki/internal/vrp"
	"io"
	"net"
	"net/http"
	"os"
	"time"
)

type Request struct {
	Action string     `json:"action"`
	Record vrp.Record `json:"record"`
}
type Status struct {
	Session     uint16       `json:"session"`
	Serial      uint32       `json:"serial"`
	Records     int          `json:"records"`
	Clients     []rtr.Client `json:"clients"`
	TLSListener string       `json:"tlsListener,omitempty"`
	Version     uint8        `json:"rtrVersion"`
	Refresh     uint32       `json:"refreshSeconds"`
	Retry       uint32       `json:"retrySeconds"`
	Expire      uint32       `json:"expireSeconds"`
}
type API struct {
	Store *vrp.Store
	RTR   *rtr.Server
	TLS   *rtr.Server
}

func (a API) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method != "POST" {
		http.Error(w, "POST required", 405)
		return
	}
	var q Request
	r.Body = http.MaxBytesReader(w, r.Body, 4096)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&q); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	if err := dec.Decode(new(any)); err != io.EOF {
		http.Error(w, "expected one JSON request", 400)
		return
	}
	var result any
	var err error
	switch q.Action {
	case "address":
		result, err = a.address()
	case "status":
		session, serial, rs := a.Store.State()
		clients := a.RTR.Clients()
		tlsListener := ""
		if a.TLS != nil {
			clients = append(clients, a.TLS.Clients()...)
			tlsListener = a.TLS.ListenerAddress()
		}
		result = Status{
			Session: session, Serial: serial, Records: len(rs), Clients: clients, TLSListener: tlsListener,
			Version: a.RTR.RTRVersion(), Refresh: a.RTR.RefreshInterval(), Retry: rtr.RetryInterval, Expire: rtr.ExpireInterval,
		}
	case "list":
		_, _, result = a.Store.State()
	case "add":
		err = a.Store.Edit(q.Record, true)
	case "remove":
		err = a.Store.Edit(q.Record, false)
	case "clear":
		err = a.Store.Replace([]vrp.Record{})
	default:
		err = fmt.Errorf("unknown action")
	}
	if err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	if result == nil {
		_, serial, rs := a.Store.State()
		result = map[string]any{"serial": serial, "vrps": rs}
		if q.Action == "add" || q.Action == "remove" {
			result.(map[string]any)["record"] = q.Record
		}
	}
	json.NewEncoder(w).Encode(result)
}
func Listen(path string) (net.Listener, error) {
	l, err := net.Listen("unix", path)
	if err != nil {
		return nil, err
	}
	if err = os.Chmod(path, 0600); err != nil {
		l.Close()
		return nil, err
	}
	return l, nil
}
func Call(ctx context.Context, path string, q Request) ([]byte, error) {
	t := &http.Transport{DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, "unix", path)
	}}
	defer t.CloseIdleConnections()
	buff := new(bytes.Buffer)
	if err := json.NewEncoder(buff).Encode(q); err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, "POST", "http://localhost/control", buff)
	if err != nil {
		return nil, err
	}
	res, err := (&http.Client{Transport: t, Timeout: 10 * time.Second}).Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	b, err := io.ReadAll(io.LimitReader(res.Body, 16<<20))
	if res.StatusCode != 200 {
		return nil, fmt.Errorf("%s", b)
	}
	return b, err
}
