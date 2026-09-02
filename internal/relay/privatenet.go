package relay

import "net"

// IsPrivate reports whether ip is a destination Rookery refuses to dial by
// default: RFC1918/RFC4193 private ranges, loopback, or link-local.
func IsPrivate(ip net.IP) bool {
	return ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast()
}
