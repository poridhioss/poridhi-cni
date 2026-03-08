package vxlan

import (
	"fmt"
	"net"
	"syscall"

	"github.com/vishvananda/netlink"
)

// FDBEntry represents a forwarding database entry
type FDBEntry struct {
	MAC       net.HardwareAddr // Container MAC address
	RemoteIP  net.IP           // Remote VTEP IP
	VXLANName string           // VXLAN interface name
}

// AddFDBEntry adds an FDB entry to forward MAC to remote VTEP
func AddFDBEntry(entry *FDBEntry) error {
	link, err := netlink.LinkByName(entry.VXLANName)
	if err != nil {
		return fmt.Errorf("vxlan interface not found: %w", err)
	}

	neigh := &netlink.Neigh{
		LinkIndex:    link.Attrs().Index,
		Family:       syscall.AF_BRIDGE,
		State:        netlink.NUD_PERMANENT,
		Flags:        netlink.NTF_SELF,
		IP:           entry.RemoteIP,
		HardwareAddr: entry.MAC,
	}

	if err := netlink.NeighAppend(neigh); err != nil {
		return fmt.Errorf("failed to add FDB entry: %w", err)
	}

	return nil
}

// DeleteFDBEntry removes an FDB entry
func DeleteFDBEntry(entry *FDBEntry) error {
	link, err := netlink.LinkByName(entry.VXLANName)
	if err != nil {
		return nil // Interface doesn't exist
	}

	neigh := &netlink.Neigh{
		LinkIndex:    link.Attrs().Index,
		Family:       syscall.AF_BRIDGE,
		IP:           entry.RemoteIP,
		HardwareAddr: entry.MAC,
	}

	return netlink.NeighDel(neigh)
}

// ListFDBEntries returns all FDB entries for a VXLAN interface
func ListFDBEntries(vxlanName string) ([]FDBEntry, error) {
	link, err := netlink.LinkByName(vxlanName)
	if err != nil {
		return nil, err
	}

	neighs, err := netlink.NeighList(link.Attrs().Index, syscall.AF_BRIDGE)
	if err != nil {
		return nil, err
	}

	var entries []FDBEntry
	for _, n := range neighs {
		if n.HardwareAddr != nil {
			entries = append(entries, FDBEntry{
				MAC:       n.HardwareAddr,
				RemoteIP:  n.IP,
				VXLANName: vxlanName,
			})
		}
	}

	return entries, nil
}

// AddBroadcastEntry adds a broadcast FDB entry (for BUM traffic)
// This forwards broadcast/unknown/multicast to remote VTEPs
func AddBroadcastEntry(vxlanName string, remoteIP net.IP) error {
	// All-zeros MAC is the default/broadcast entry
	broadcastMAC, _ := net.ParseMAC("00:00:00:00:00:00")

	return AddFDBEntry(&FDBEntry{
		MAC:       broadcastMAC,
		RemoteIP:  remoteIP,
		VXLANName: vxlanName,
	})
}

// FlushFDB removes all FDB entries for a VXLAN interface
func FlushFDB(vxlanName string) error {
	entries, err := ListFDBEntries(vxlanName)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		DeleteFDBEntry(&entry)
	}

	return nil
}


