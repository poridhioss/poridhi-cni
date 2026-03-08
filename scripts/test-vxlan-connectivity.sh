#!/bin/bash

set -e

ACTION=${1:-create}
NUM_CONTAINERS=${2:-2}

BRIDGE="cni0"
NS_PREFIX="vxlan-test"

create_containers() {
    BRIDGE_IP=$(ip addr show $BRIDGE | grep "inet " | awk '{print $2}' | cut -d'/' -f1)
    SUBNET_BASE=$(echo $BRIDGE_IP | cut -d'.' -f1-3)

    echo "=== Creating $NUM_CONTAINERS Test Containers ==="
    echo "Bridge: $BRIDGE ($BRIDGE_IP)"
    echo ""

    for i in $(seq 1 $NUM_CONTAINERS); do
        NS="${NS_PREFIX}-$i"
        VETH_HOST="veth-vx-$i"
        VETH_CONT="veth-c-$i"
        IP="${SUBNET_BASE}.$((i+1))"

        echo "Creating $NS with IP $IP..."

        # Create namespace
        ip netns add $NS

        # Create veth pair (use temporary name to avoid conflict with host eth0)
        ip link add $VETH_HOST type veth peer name $VETH_CONT

        # Attach host end to bridge
        ip link set $VETH_HOST master $BRIDGE
        ip link set $VETH_HOST up

        # Move container end to namespace and rename to eth0
        ip link set $VETH_CONT netns $NS
        ip netns exec $NS ip link set $VETH_CONT name eth0

        # Configure container
        ip netns exec $NS ip addr add ${IP}/24 dev eth0
        ip netns exec $NS ip link set eth0 up
        ip netns exec $NS ip link set lo up
        ip netns exec $NS ip route add default via $BRIDGE_IP

        # Show MAC (needed for FDB on remote)
        MAC=$(ip netns exec $NS cat /sys/class/net/eth0/address)
        echo "  IP: $IP  MAC: $MAC"
    done

    echo ""
    echo "=== Local Connectivity Test ==="

    for i in $(seq 1 $NUM_CONTAINERS); do
        SRC_IP="${SUBNET_BASE}.$((i+1))"

        for j in $(seq 1 $NUM_CONTAINERS); do
            if [ $i -ne $j ]; then
                DST_IP="${SUBNET_BASE}.$((j+1))"

                if ip netns exec ${NS_PREFIX}-$i ping -c 1 -W 1 $DST_IP > /dev/null 2>&1; then
                    echo "  PASS: $SRC_IP -> $DST_IP"
                else
                    echo "  FAIL: $SRC_IP -> $DST_IP"
                fi
            fi
        done
    done

    echo ""
    echo "=== FDB Entries for Remote Host ==="
    echo "Add these on the remote host (replace <THIS_HOST_IP> with this host's private IP):"
    echo ""
    for i in $(seq 1 $NUM_CONTAINERS); do
        NS="${NS_PREFIX}-$i"
        MAC=$(ip netns exec $NS cat /sys/class/net/eth0/address)
        echo "  bridge fdb append $MAC dev vxlan100 dst <THIS_HOST_IP>"
    done
    echo ""
}

cleanup_containers() {
    echo "=== Cleaning Up Test Containers ==="
    for i in $(seq 1 10); do
        ip netns del ${NS_PREFIX}-$i 2>/dev/null && echo "  Deleted ${NS_PREFIX}-$i" || true
        ip link del veth-vx-$i 2>/dev/null || true
    done
    echo "Done."
}

case $ACTION in
    create)
        cleanup_containers
        echo ""
        create_containers
        ;;
    cleanup)
        cleanup_containers
        ;;
    *)
        echo "Usage: $0 [create|cleanup] [num-containers]"
        echo ""
        echo "  create  - Create test containers attached to cni0 bridge"
        echo "  cleanup - Remove all test containers"
        exit 1
        ;;
esac