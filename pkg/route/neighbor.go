package route

import (
	"fmt"
	"net"

	"github.com/vishvananda/netlink"
)

// NeighborInfo represents a neighbor cache entry
type NeighborInfo struct {
	IP    string
	MAC   string
	State string
}

// AddNeighbor adds a static ARP/neighbor entry
func AddNeighbor(ip net.IP, mac net.HardwareAddr, linkName string) error {
	link, err := netlink.LinkByName(linkName)
	if err != nil {
		return fmt.Errorf("failed to get link %s: %w", linkName, err)
	}

	neigh := &netlink.Neigh{
		LinkIndex:    link.Attrs().Index,
		IP:           ip,
		HardwareAddr: mac,
		State:        netlink.NUD_PERMANENT,
	}

	if err := netlink.NeighAdd(neigh); err != nil {
		return fmt.Errorf("failed to add neighbor: %w", err)
	}

	return nil
}

// DeleteNeighbor removes a neighbor entry
func DeleteNeighbor(ip net.IP, linkName string) error {
	link, err := netlink.LinkByName(linkName)
	if err != nil {
		return fmt.Errorf("failed to get link %s: %w", linkName, err)
	}

	neigh := &netlink.Neigh{
		LinkIndex: link.Attrs().Index,
		IP:        ip,
	}

	if err := netlink.NeighDel(neigh); err != nil {
		return fmt.Errorf("failed to delete neighbor: %w", err)
	}

	return nil
}

// GetNeighbors returns all neighbor entries for an interface
func GetNeighbors(linkName string) ([]NeighborInfo, error) {
	link, err := netlink.LinkByName(linkName)
	if err != nil {
		return nil, fmt.Errorf("failed to get link %s: %w", linkName, err)
	}

	neighbors, err := netlink.NeighList(link.Attrs().Index, netlink.FAMILY_V4)
	if err != nil {
		return nil, fmt.Errorf("failed to list neighbors: %w", err)
	}

	var result []NeighborInfo
	for _, n := range neighbors {
		result = append(result, NeighborInfo{
			IP:    n.IP.String(),
			MAC:   n.HardwareAddr.String(),
			State: neighStateToString(n.State),
		})
	}

	return result, nil
}

func neighStateToString(state int) string {
	switch state {
	case netlink.NUD_NONE:
		return "NONE"
	case netlink.NUD_INCOMPLETE:
		return "INCOMPLETE"
	case netlink.NUD_REACHABLE:
		return "REACHABLE"
	case netlink.NUD_STALE:
		return "STALE"
	case netlink.NUD_DELAY:
		return "DELAY"
	case netlink.NUD_PROBE:
		return "PROBE"
	case netlink.NUD_FAILED:
		return "FAILED"
	case netlink.NUD_NOARP:
		return "NOARP"
	case netlink.NUD_PERMANENT:
		return "PERMANENT"
	default:
		return fmt.Sprintf("UNKNOWN(%d)", state)
	}
}

// FlushNeighbors removes all neighbor entries for an interface
func FlushNeighbors(linkName string) error {
	neighbors, err := GetNeighbors(linkName)
	if err != nil {
		return err
	}

	for _, n := range neighbors {
		ip := net.ParseIP(n.IP)
		if ip != nil {
			// Ignore errors - some entries may not be deletable
			DeleteNeighbor(ip, linkName)
		}
	}

	return nil
}
