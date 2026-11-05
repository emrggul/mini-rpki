package rtr

import (
	"bytes"
	"encoding/hex"
	"testing"
)

func TestMalformedEncapsulation(t *testing.T) {
	for _, raw := range []string{"0102000000000007", "0102000000010000", "010100000000000c0012"} {
		b, _ := hex.DecodeString(raw)
		p, err := Read(bytes.NewReader(b))
		if err == nil {
			t.Fatal("expected error")
		}
		report := Error(0, p, "bad input")
		n := int(be.Uint32(report.Body))
		if !bytes.Equal(report.Body[4:4+n], b) {
			t.Fatalf("encapsulation changed input: %x", report.Body)
		}
	}
}
