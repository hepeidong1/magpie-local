package provider

import (
	"net"

	"github.com/yetone/magpie/internal/netproxy"
)

// LocalHost says host is on this machine or the local network: localhost,
// a .local name, a loopback, private or link-local IP, or a name every
// address of which is one (netproxy.LocalName, looked up now and kept a
// minute; one that doesn't resolve isn't). Only the editor's caution asks
// it (/api/local-hosts), never a request.
func LocalHost(host string) bool {
	if local(host) {
		return true
	}
	if ip := net.ParseIP(host); ip != nil {
		return netproxy.LocalIP(ip)
	}
	return netproxy.LocalName(host)
}
