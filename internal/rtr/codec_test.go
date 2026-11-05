package rtr

import (
	"bytes"
	"encoding/hex"
	"github.com/minirpki/mini-rpki/internal/vrp"
	"io"
	"net/netip"
	"testing"
)

func TestPrefixEncoding(t *testing.T) {
	for _, tt := range []struct {
		p    string
		m    uint8
		want string
	}{{"192.0.2.0/24", 24, "010400000000001401181800c00002000000fde9"}, {"2001:db8:100::/48", 48, "01060000000000200130300020010db80100000000000000000000000000fde9"}} {
		r := vrp.Record{Prefix: netip.MustParsePrefix(tt.p), MaxLen: tt.m, ASN: 65001}
		b := Prefix(r, true).Bytes()
		if hex.EncodeToString(b) != tt.want {
			t.Fatalf("%x != %s", b, tt.want)
		}
		p, e := Read(bytes.NewReader(b))
		if e != nil || !bytes.Equal(p.Bytes(), b) {
			t.Fatal(p, e)
		}
		if Prefix(r, false).Body[0] != 0 {
			t.Fatal("withdrawal flag")
		}
	}
}
func TestVersionZeroEndOfDataEncoding(t *testing.T) {
	p := EndOfDataVersion(0, 42, 99, 5, 10, 600)
	if got, want := hex.EncodeToString(p.Bytes()), "0007002a0000000c00000063"; got != want {
		t.Fatalf("%s != %s", got, want)
	}
}

type slowReader struct{ io.Reader }

func (r slowReader) Read(b []byte) (int, error) {
	if len(b) > 1 {
		b = b[:1]
	}
	return r.Reader.Read(b)
}

type slowWriter struct{ bytes.Buffer }

func (w *slowWriter) Write(b []byte) (int, error) {
	if len(b) > 1 {
		b = b[:1]
	}
	return w.Buffer.Write(b)
}
func TestPartialIO(t *testing.T) {
	p := New(SerialQuery, 42, 99)
	got, e := Read(slowReader{bytes.NewReader(p.Bytes())})
	if e != nil || !bytes.Equal(got.Bytes(), p.Bytes()) {
		t.Fatal(e)
	}
	w := &slowWriter{}
	if e = Write(w, p); e != nil || !bytes.Equal(w.Bytes(), p.Bytes()) {
		t.Fatal(e)
	}
}
func TestMalformed(t *testing.T) {
	for _, h := range []string{"", "0102", "0102000000000007", "0102000000001001", "010100000000000c0000"} {
		b, _ := hex.DecodeString(h)
		if _, e := Read(bytes.NewReader(b)); e == nil {
			t.Fatalf("accepted %s", h)
		}
	}
}
func TestErrorEncoding(t *testing.T) {
	p := Error(4, New(ResetQuery, 0), "unsupported")
	if p.Field != 4 || be.Uint32(p.Body) != 8 || be.Uint32(p.Body[12:]) != 11 {
		t.Fatal(p)
	}
}
