package vrp

import "net/netip"

// DemoRecords is the small documentation-only dataset used on first startup.
func DemoRecords() []Record {
	return []Record{
		{Prefix: netip.MustParsePrefix("192.0.2.0/24"), MaxLen: 24, ASN: 65001},
		{Prefix: netip.MustParsePrefix("2001:db8:100::/48"), MaxLen: 48, ASN: 65001},
	}
}
