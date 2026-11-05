package scenario

import (
	"github.com/minirpki/mini-rpki/internal/vrp"
	"net/netip"
)

type Scenario struct {
	Name        string       `json:"name"`
	Records     []vrp.Record `json:"vrps"`
	Route       string       `json:"advertisePrefix"`
	ASN         uint32       `json:"originASN"`
	Expected    string       `json:"expectedState"`
	Explanation string       `json:"explanation"`
}

func record(p string, m uint8, a uint32) vrp.Record {
	return vrp.Record{Prefix: netip.MustParsePrefix(p), MaxLen: m, ASN: a}
}
func All() []Scenario {
	return []Scenario{
		{"valid", []vrp.Record{record("192.0.2.0/24", 24, 65001), record("2001:db8:100::/48", 48, 65001)}, "192.0.2.0/24", 65001, "Valid", "A covering VRP authorizes this origin and prefix length."},
		{"invalid-asn", []vrp.Record{record("192.0.2.0/24", 24, 65002)}, "192.0.2.0/24", 65001, "Invalid", "A covering VRP exists but does not authorize AS65001."},
		{"invalid-length", []vrp.Record{record("192.0.2.0/24", 24, 65001)}, "192.0.2.0/25", 65001, "Invalid", "The /25 exceeds the authorized maximum length of /24."},
		{"not-found", []vrp.Record{}, "192.0.2.0/24", 65001, "Not Found", "No VRP covers this route."},
		{"multiple-origins", []vrp.Record{record("192.0.2.0/24", 24, 65001), record("192.0.2.0/24", 24, 65002)}, "192.0.2.0/24", 65001, "Valid", "Either AS65001 or AS65002 is authorized; one matching VRP is sufficient."},
	}
}
func Find(name string) (Scenario, bool) {
	for _, s := range All() {
		if s.Name == name {
			return s, true
		}
	}
	return Scenario{}, false
}
