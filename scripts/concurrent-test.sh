#!/bin/bash

set -e

# Configuration
CNI_PLUGIN="./poridhi-cni"
CNI_CONFIG="/tmp/cni-concurrent.conf"
NETWORK_NAME="concurrent-test-net"
NUM_CONCURRENT=${1:-20}

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

echo "========================================"
echo "  Concurrent IPAM Stress Test"
echo "  Parallel containers: $NUM_CONCURRENT"
echo "========================================"
echo ""

# Disable bridge netfilter for container connectivity
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

# Cleanup function
cleanup() {
    echo ""
    echo "Cleaning up..."

    for i in $(seq 1 $NUM_CONCURRENT); do
        container_id="cc$i"
        ns_path="/var/run/netns/$container_id"

        if [ -e "$ns_path" ]; then
            cat $CNI_CONFIG | \
                CNI_COMMAND=DEL \
                CNI_CONTAINERID=$container_id \
                CNI_NETNS=$ns_path \
                CNI_IFNAME=eth0 \
                CNI_PATH=/opt/cni/bin \
                $CNI_PLUGIN 2>/dev/null || true

            sudo ip netns del $container_id 2>/dev/null || true
        fi
    done

    sudo rm -rf /var/lib/cni/networks/$NETWORK_NAME 2>/dev/null || true
    rm -f /tmp/result-*.json 2>/dev/null || true
    rm -f /tmp/all-ips.txt 2>/dev/null || true

    echo "Cleanup complete"
}

trap cleanup EXIT
cleanup 2>/dev/null || true

echo "Step 1: Creating $NUM_CONCURRENT containers in parallel..."
echo ""

for i in $(seq 1 $NUM_CONCURRENT); do
    (
        container_id="cc$i"
        ns_path="/var/run/netns/$container_id"

        # Create namespace
        sudo ip netns add $container_id 2>/dev/null

        # Run CNI ADD and save result
        cat $CNI_CONFIG | \
            CNI_COMMAND=ADD \
            CNI_CONTAINERID=$container_id \
            CNI_NETNS=$ns_path \
            CNI_IFNAME=eth0 \
            CNI_PATH=/opt/cni/bin \
            $CNI_PLUGIN > /tmp/result-$i.json 2>&1
    ) &
done

# Wait for all background jobs to complete
echo "Waiting for all containers to be created..."
wait
echo -e "${GREEN}All containers created.${NC}"
echo ""

echo "Step 2: Validating IP allocations..."
echo ""

> /tmp/all-ips.txt
success_count=0
fail_count=0

for i in $(seq 1 $NUM_CONCURRENT); do
    if [ -f /tmp/result-$i.json ]; then
        # Try to extract IP from JSON result
        ip=$(cat /tmp/result-$i.json | jq -r '.ips[0].address' 2>/dev/null | cut -d'/' -f1)

        if [ "$ip" != "null" ] && [ -n "$ip" ] && [ "$ip" != "" ]; then
            echo "$ip" >> /tmp/all-ips.txt
            echo -e "  ${GREEN}✓${NC} cc$i: $ip"
            ((success_count++)) || true
        else
            echo -e "  ${RED}✗${NC} cc$i: Failed to get IP"
            echo "    Result: $(cat /tmp/result-$i.json)"
            ((fail_count++)) || true
        fi
    else
        echo -e "  ${RED}✗${NC} cc$i: No result file"
        ((fail_count++)) || true
    fi
done

echo ""
echo "Allocations: $success_count successful, $fail_count failed"
echo ""

echo "Step 3: Checking for duplicate IPs..."
echo ""

duplicates=$(sort /tmp/all-ips.txt | uniq -d)

if [ -n "$duplicates" ]; then
    echo -e "${RED}ERROR: Duplicate IPs found!${NC}"
    echo "$duplicates"
    echo ""
    echo "This indicates a race condition in IPAM locking!"
    exit 1
else
    unique_count=$(sort /tmp/all-ips.txt | uniq | wc -l)
    echo -e "${GREEN}SUCCESS: $unique_count unique IPs allocated, no duplicates!${NC}"
fi

echo ""
echo "Step 4: IP allocation summary..."
echo ""
echo "Allocated IPs (sorted):"
sort -t. -k4 -n /tmp/all-ips.txt | head -20
if [ $success_count -gt 20 ]; then
    echo "... and $((success_count - 20)) more"
fi

echo ""
echo "========================================"
echo "  Results"
echo "========================================"
echo ""
echo "Containers attempted: $NUM_CONCURRENT"
echo "Successful allocations: $success_count"
echo "Failed allocations: $fail_count"
echo "Duplicate IPs: $(echo "$duplicates" | grep -c . || echo 0)"
echo ""

if [ $fail_count -eq 0 ] && [ -z "$duplicates" ]; then
    echo -e "${GREEN}IPAM stress test passed!${NC}"
    exit 0
else
    echo -e "${RED}IPAM stress test failed!${NC}"
    exit 1
fi
