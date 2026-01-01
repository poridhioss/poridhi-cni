package ipam

import (
	"encoding/binary"
	"net"
)

// ParseCIDR parses a CIDR string and returns network info
func ParseCIDR(cidr string) (*net.IPNet, net.IP, net.IP, error) {
	ip, ipnet, err := net.ParseCIDR(cidr)
	if err != nil {
		return nil, nil, nil, err
	}

	// Calculate first and last usable IPs
	first := NextIP(ip.Mask(ipnet.Mask)) // Skip network address
	last := LastIP(ipnet)

	return ipnet, first, last, nil
}

// NextIP returns the next IP address
func NextIP(ip net.IP) net.IP {
	next := make(net.IP, len(ip))
	copy(next, ip)

	for i := len(next) - 1; i >= 0; i-- {
		next[i]++
		if next[i] > 0 {
			break
		}
	}
	return next
}

// LastIP returns the last usable IP in the subnet (before broadcast)
func LastIP(ipnet *net.IPNet) net.IP {
	ip := ipnet.IP.To4()
	mask := ipnet.Mask

	last := make(net.IP, 4)
	for i := 0; i < 4; i++ {
		last[i] = ip[i] | ^mask[i]
	}
	// Subtract 1 to exclude broadcast address
	last[3]--
	return last
}

// IPToUint32 converts IP to uint32 for arithmetic
func IPToUint32(ip net.IP) uint32 {
	ip = ip.To4()
	return binary.BigEndian.Uint32(ip)
}

// Uint32ToIP converts uint32 back to IP
func Uint32ToIP(n uint32) net.IP {
	ip := make(net.IP, 4)
	binary.BigEndian.PutUint32(ip, n)
	return ip
}

// IPInRange checks if IP is within the subnet
func IPInRange(ip net.IP, ipnet *net.IPNet) bool {
	return ipnet.Contains(ip)
}