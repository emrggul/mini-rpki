package scenario

import (
	"github.com/minirpki/mini-rpki/internal/vrp"
	"net/netip"
	"testing"
)

func TestScenarios(t *testing.T) {
	for _, s := range All() {
		t.Run(s.Name, func(t *testing.T) {
			if _, err := vrp.Normalize(s.Records); err != nil {
				t.Fatal(err)
			}
			if got := vrp.ValidateRoute(s.Records, netip.MustParsePrefix(s.Route), s.ASN); got != s.Expected {
				t.Fatalf("got %s expected %s", got, s.Expected)
			}
		})
	}
	s, _ := Find("valid")
	if vrp.ValidateRoute(s.Records, netip.MustParsePrefix("2001:db8:100::/48"), 65001) != "Valid" {
		t.Fatal("IPv6")
	}
}
