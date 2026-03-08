#!/bin/bash

# Manual VXLAN setup between two hosts
# Run this on each host with appropriate parameters

set -e

usage() {
    echo "Usage: $0 <local-ip> <remote-ip> <local-subnet> <remote-subnet>"
    echo ""
    echo "Example (Host A):"
    echo "  $0 10.0.1.10 10.0.1.20 10.244.0.0/24 10.244.1.0/24"
    echo ""
    echo "Example (Host B):"
    echo "  $0 10.0.1.20 10.0.1.10 10.244.1.0/24 10.244.0.0/24"
    exit 1
}

if [ $# -ne 4 ]; then
    usage
fi

LOCAL_IP=$1
REMOTE_IP=$2
LOCAL_SUBNET=$3
REMOTE_SUBNET=$4

VNI=100
VXLAN_NAME="vxlan${VNI}"
BRIDGE_NAME="cni0"

echo "=== VXLAN Manual Setup ==="
echo "Local IP: $LOCAL_IP"
echo "Remote IP: $REMOTE_IP"
echo "Local Subnet: $LOCAL_SUBNET"
echo "Remote Subnet: $REMOTE_SUBNET"
echo ""

# Calculate gateway IP from subnet (first usable IP)
LOCAL_GW=$(echo $LOCAL_SUBNET | sed 's/\.0\//.1\//' | cut -d'/' -f1)

# Cleanup existing setup
echo "Cleaning up existing setup..."
ip link del $VXLAN_NAME 2>/dev/null || true
ip link del $BRIDGE_NAME 2>/dev/null || true

# 1. Create bridge
echo "1. Creating bridge $BRIDGE_NAME..."
ip link add $BRIDGE_NAME type bridge
ip addr add ${LOCAL_GW}/24 dev $BRIDGE_NAME
ip link set $BRIDGE_NAME up

# 2. Create VXLAN interface
echo "2. Creating VXLAN interface $VXLAN_NAME (VNI=$VNI)..."
ip link add $VXLAN_NAME type vxlan \
    id $VNI \
    local $LOCAL_IP \
    dstport 4789 \
    nolearning

# 3. Attach VXLAN to bridge
echo "3. Attaching VXLAN to bridge..."
ip link set $VXLAN_NAME master $BRIDGE_NAME
ip link set $VXLAN_NAME up

# 4. Add FDB entry for remote VTEP (broadcast entry)
echo "4. Adding FDB entry for remote VTEP..."
bridge fdb append 00:00:00:00:00:00 dev $VXLAN_NAME dst $REMOTE_IP

# 5. Add route to remote container subnet
echo "5. Adding route to remote subnet..."
ip route add $REMOTE_SUBNET dev $BRIDGE_NAME 2>/dev/null || true

# 6. Enable IP forwarding
echo "6. Enabling IP forwarding..."
echo 1 > /proc/sys/net/ipv4/ip_forward

echo ""
echo "=== Setup Complete ==="
echo ""
echo "Bridge: $BRIDGE_NAME"
ip addr show $BRIDGE_NAME | grep inet

echo ""
echo "VXLAN: $VXLAN_NAME"
ip -d link show $VXLAN_NAME | head -5

echo ""
echo "FDB entries:"
bridge fdb show dev $VXLAN_NAME

echo ""
echo "Routes:"
ip route | grep -E "($LOCAL_SUBNET|$REMOTE_SUBNET)" || echo "  (check routes manually)"

echo ""
echo "=== Next Steps ==="
echo "1. Run this script on the remote host with swapped parameters"
echo "2. Create test containers: sudo ~/test-vxlan-connectivity.sh create 2"
echo "3. Test cross-host connectivity"
