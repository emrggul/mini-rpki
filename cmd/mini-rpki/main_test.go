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
