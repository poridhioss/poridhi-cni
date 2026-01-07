package cni

// NetConf represents the CNI network configuration received from the runtime
type NetConf struct {
	CNIVersion string `json:"cniVersion"`
	Name       string `json:"name"`
	Type       string `json:"type"`
	Bridge     string `json:"bridge,omitempty"`
	MTU        int    `json:"mtu,omitempty"`
	IPAM       IPAM   `json:"ipam,omitempty"`
	DNS        DNS    `json:"dns,omitempty"`
}

// IPAM holds IP Address Management configuration
type IPAM struct {
	Type    string `json:"type"`
	Subnet  string `json:"subnet"`
	Gateway string `json:"gateway,omitempty"`
}

// DNS holds DNS configuration for the container
type DNS struct {
	Nameservers []string `json:"nameservers,omitempty"`
	Search      []string `json:"search,omitempty"`
}

// Result represents the CNI result returned to the runtime
type Result struct {
	CNIVersion string      `json:"cniVersion"`
	Interfaces []Interface `json:"interfaces,omitempty"`
	IPs        []IPConfig  `json:"ips,omitempty"`
	Routes     []Route     `json:"routes,omitempty"`
	DNS        DNS         `json:"dns,omitempty"`
}

// Interface describes a network interface created by the plugin
type Interface struct {
	Name    string `json:"name"`
	Mac     string `json:"mac,omitempty"`
	Sandbox string `json:"sandbox,omitempty"`
}

// IPConfig holds IP address configuration
type IPConfig struct {
	Address   string `json:"address"`
	Gateway   string `json:"gateway,omitempty"`
	Interface int    `json:"interface"`
}

// Route represents a routing rule
type Route struct {
	Dst string `json:"dst"`
	GW  string `json:"gw,omitempty"`
}

// Error represents a CNI error response
type Error struct {
	Code    int    `json:"code"`
	Msg     string `json:"msg"`
	Details string `json:"details,omitempty"`
}