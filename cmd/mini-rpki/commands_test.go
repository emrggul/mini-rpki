package main

import (
	"testing"
)

func TestSimpleCommands(t *testing.T) {
	for _, tc := range []struct {
		args []string
		want uint8
	}{
		{[]string{"add", "192.0.2.0/24", "--asn", "65001"}, 24},
		{[]string{"remove", "192.0.2.0/24", "--asn", "65001"}, 24},
		{[]string{"add", "2001:db8:100::/48", "--asn", "65001"}, 48},
		{[]string{"add", "192.0.2.0/24", "--asn", "65001", "--maxlen", "25"}, 25},
		{[]string{"vrp", "add", "192.0.2.0/24", "--asn", "65001", "--maxlen", "25"}, 25},
	} {
		q, err := parseRequest(tc.args)
		if err != nil {
			t.Fatal(tc.args, err)
		}
		if q.Record.MaxLen != tc.want || q.Record.ASN != 65001 {
			t.Fatal(q)
		}
	}
	for _, action := range []string{"list", "clear", "status", "interface"} {
		q, err := parseRequest([]string{action})
		if err != nil || q.Action != action {
			t.Fatal(q, err)
		}
	}
}
func TestRemovedCommands(t *testing.T) {
	for _, args := range [][]string{{"scenario", "list"}, {"scenario", "apply", "valid"}, {"add", "192.0.2.0/24"}, {"add", "192.0.2.0/24", "--asn", "65001", "--maxlen", "23"}, {"add", "2001:db8::/32", "--asn", "65001", "--maxlen", "129"}} {
		if _, err := parseRequest(args); err == nil {
			t.Fatal("accepted", args)
		}
	}
}
