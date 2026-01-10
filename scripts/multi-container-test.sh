#!/bin/bash

set -e

# Configuration
CNI_PLUGIN="./poridhi-cni"
CNI_CONFIG="/tmp/cni-multi-test.conf"
NETWORK_NAME="multi-test-net"
NUM_CONTAINERS=${1:-5}
NO_CLEANUP=${2:-false}  # Pass "true" as second arg to skip cleanup

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo "========================================"
echo "  Multi-Container CNI Test"
echo "  Containers: $NUM_CONTAINERS"
echo "========================================"
echo ""

# Disable bridge netfilter to allow container-to-container traffic
# Without this, iptables FORWARD chain (policy DROP) blocks bridge traffic
# Production solution: Add iptables ACCEPT rules (covered in Lab 09)
if [ -f /proc/sys/net/bridge/bridge-nf-call-iptables ]; then
    echo 0 > /proc/sys/net/bridge/bridge-nf-call-iptables 2>/dev/null || true
fi

# Create CNI configuration
cat > $CNI_CONFIG << EOF
{
  "cniVersion": "1.0.0",
  "name": "$NETWORK_NAME",
  "type": "poridhi-cni",
  "bridge": "cni0",
  "ipam": {
    "subnet": "10.244.0.0/24",
    "gateway": "10.244.0.1"
  }
}
EOF

cleanup() {
    echo ""
    echo "Cleaning up..."

    for i in $(seq 1 $NUM_CONTAINERS); do
        container_id="ctr$i"
        ns_path="/var/run/netns/$container_id"

        if [ -e "$ns_path" ]; then
            # Run CNI DEL
            cat $CNI_CONFIG | \
                CNI_COMMAND=DEL \
                CNI_CONTAINERID=$container_id \
                CNI_NETNS=$ns_path \
                CNI_IFNAME=eth0 \
                CNI_PATH=/opt/cni/bin \
                $CNI_PLUGIN 2>/dev/null || true

            # Delete namespace
            sudo ip netns del $container_id 2>/dev/null || true
        fi
    done

    # Clean IPAM state
    sudo rm -rf /var/lib/cni/networks/$NETWORK_NAME 2>/dev/null || true

    echo "Cleanup complete"
}

# Set trap to cleanup on exit (unless --no-cleanup)
if [ "$NO_CLEANUP" != "true" ]; then
    trap cleanup EXIT
fi

# Initial cleanup
cleanup 2>/dev/null || true

echo "Step 1: Creating $NUM_CONTAINERS containers..."
echo ""

declare -a CONTAINER_IPS

for i in $(seq 1 $NUM_CONTAINERS); do
    container_id="ctr$i"
    ns_path="/var/run/netns/$container_id"

    # Create namespace
    sudo ip netns add $container_id

    # Run CNI ADD
    result=$(cat $CNI_CONFIG | \
        CNI_COMMAND=ADD \
        CNI_CONTAINERID=$container_id \
        CNI_NETNS=$ns_path \
        CNI_IFNAME=eth0 \
        CNI_PATH=/opt/cni/bin \
        $CNI_PLUGIN)

    # Extract IP from result
    ip=$(echo $result | jq -r '.ips[0].address' | cut -d'/' -f1)
    CONTAINER_IPS[$i]=$ip

    echo -e "  ${GREEN}✓${NC} $container_id: $ip"
done

echo ""
echo "Step 2: Testing gateway connectivity..."
echo ""

gateway_passed=0
gateway_failed=0

for i in $(seq 1 $NUM_CONTAINERS); do
    container_id="ctr$i"

    if sudo ip netns exec $container_id ping -c 1 -W 2 10.244.0.1 > /dev/null 2>&1; then
        echo -e "  ${GREEN}✓${NC} $container_id → gateway (10.244.0.1)"
        ((gateway_passed++)) || true
    else
        echo -e "  ${RED}✗${NC} $container_id → gateway (10.244.0.1)"
        ((gateway_failed++)) || true
    fi
done

echo ""
echo "Gateway results: $gateway_passed passed, $gateway_failed failed"

echo ""
echo "Step 3: Testing full-mesh connectivity..."
echo ""

mesh_passed=0
mesh_failed=0

for i in $(seq 1 $NUM_CONTAINERS); do
    src_container="ctr$i"
    src_ip=${CONTAINER_IPS[$i]}

    for j in $(seq 1 $NUM_CONTAINERS); do
        if [ $i -ne $j ]; then
            dst_container="ctr$j"
            dst_ip=${CONTAINER_IPS[$j]}

            # Ping test
            if sudo ip netns exec $src_container ping -c 1 -W 2 $dst_ip > /dev/null 2>&1; then
                echo -e "  ${GREEN}✓${NC} $src_container ($src_ip) → $dst_container ($dst_ip)"
                ((mesh_passed++)) || true
            else
                echo -e "  ${RED}✗${NC} $src_container ($src_ip) → $dst_container ($dst_ip)"
                ((mesh_failed++)) || true
            fi
        fi
    done
done

echo ""
echo "Mesh results: $mesh_passed passed, $mesh_failed failed"

echo ""
echo "========================================"
echo "  Summary"
echo "========================================"
echo ""
echo "Containers created: $NUM_CONTAINERS"
echo "Gateway tests: $gateway_passed/$NUM_CONTAINERS passed"
echo "Mesh tests: $mesh_passed/$((NUM_CONTAINERS * (NUM_CONTAINERS - 1))) passed"
echo ""

if [ "$NO_CLEANUP" = "true" ]; then
    echo -e "${YELLOW}Cleanup skipped. Containers are still running.${NC}"
    echo "To inspect: sudo ip netns exec ctr1 ip addr"
    echo "To cleanup: sudo ./scripts/multi-container-test.sh $NUM_CONTAINERS"
    echo ""
fi

if [ $gateway_failed -eq 0 ] && [ $mesh_failed -eq 0 ]; then
    echo -e "${GREEN}All tests passed!${NC}"
    exit 0
else
    echo -e "${RED}Some tests failed!${NC}"
    exit 1
fi
