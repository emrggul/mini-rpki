package control

import (
	"fmt"
	"net"
	"net/netip"
	"sort"
)

type Endpoint struct {
	IP        string `json:"ip"`
	Port      string `json:"port"`
	Interface string `json:"interface"`
}
type AddressInfo struct {
	Listener  string     `json:"listener"`
	Endpoints []Endpoint `json:"endpoints"`
}

// describeAddress reports bindings in the server's network namespace.
// Host NAT and remote-router routes cannot be inferred from these addresses.
func describeAddress(listener string) (AddressInfo, error) {
	info := AddressInfo{Listener: listener, Endpoints: []Endpoint{}}
	host, port, err := net.SplitHostPort(listener)
	if err != nil {
		return info, err
	}
	bound, err := netip.ParseAddr(host)
	if err != nil {
		return info, err
	}
	interfaces, err := net.Interfaces()
	if err != nil {
		return info, err
	}
	for _, iface := range interfaces {
		if iface.Flags&net.FlagUp == 0 {
			continue
		}
		addresses, err := iface.Addrs()
		if err != nil {
			return info, err
		}
		for _, address := range addresses {
			prefix, err := netip.ParsePrefix(address.String())
			if err != nil {
				continue
			}
			ip := prefix.Addr().Unmap()
			match := ip == bound.WithZone("").Unmap()
			if bound.IsUnspecified() {
				match = !ip.IsLoopback() && !ip.IsLinkLocalUnicast() && !ip.IsUnspecified() && (!bound.Is4() || ip.Is4())
			}
			if match {
				info.Endpoints = append(info.Endpoints, Endpoint{IP: ip.String(), Port: port, Interface: iface.Name})
			}
		}
	}
	if !bound.IsUnspecified() && len(info.Endpoints) == 0 {
		info.Endpoints = append(info.Endpoints, Endpoint{IP: host, Port: port})
	}
	sort.Slice(info.Endpoints, func(i, j int) bool { return info.Endpoints[i].IP < info.Endpoints[j].IP })
	return info, nil
}
func (a API) address() (AddressInfo, error) {
	listener := a.RTR.ListenerAddress()
	if listener == "" {
		return AddressInfo{}, fmt.Errorf("RTR listener is not ready")
	}
	return describeAddress(listener)
}
