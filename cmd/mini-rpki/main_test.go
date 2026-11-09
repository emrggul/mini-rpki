package main

import "testing"

func TestInvalidCLI(t *testing.T) {
	for _, args := range [][]string{{"bogus"}, {"status", "extra"}, {"vrp", "add", "bad", "--asn", "1", "--maxlen", "24"}, {"vrp", "add", "192.0.2.0/24", "--asn", "4294967296", "--maxlen", "24"}, {"vrp", "add", "192.0.2.0/24", "--asn", "-1", "--maxlen", "24"}, {"vrp", "add", "192.0.2.0/24", "--asn", "1", "--maxlen", "23"}, {"vrp", "add", "192.0.2.0/24", "--maxlen", "24"}, {"scenario", "apply"}} {
		if e := run(args); e == nil {
			t.Fatal(args)
		}
	}
}

func TestInvalidRefreshCLI(t *testing.T) {
	for _, value := range []string{"0", "600", "-1", "abc"} {
		if err := run([]string{"serve", "--refresh", value}); err == nil {
			t.Fatal("accepted", value)
		}
	}
}

func TestInvalidRTRVersionCLI(t *testing.T) {
	for _, value := range []string{"2", "-1", "abc"} {
		if err := run([]string{"serve", "--rtr-version", value}); err == nil {
			t.Fatal("accepted", value)
		}
	}
}

func TestInvalidTLSCLI(t *testing.T) {
	for _, args := range [][]string{
		{"serve", "--tls-cert", "server.pem"},
		{"serve", "--rtr-version", "2", "--tls-listen", ":324", "--tls-dir", "/tls"},
	} {
		if err := run(args); err == nil {
			t.Fatal("accepted", args)
		}
	}
}
