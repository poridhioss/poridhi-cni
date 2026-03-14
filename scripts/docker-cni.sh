#!/bin/bash

# Docker CNI Wrapper
# Usage: docker-cni.sh <action> <container-id>

set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
CNI_PLUGIN="$SCRIPT_DIR/poridhi-cni"
CNI_CONFIG="/etc/cni/net.d/10-poridhi.conflist"

action=$1
container_id=$2

usage() {
    echo "Usage: $0 <add|del|status> <container-id>"
    echo ""
    echo "Actions:"
    echo "  add      - Add CNI networking to container"
    echo "  del      - Remove CNI networking from container"
    echo "  status   - Show container network status"
    exit 1
}

if [ -z "$action" ] || [ -z "$container_id" ]; then
    usage
fi

# Get container PID and network namespace
get_netns() {
    local pid=$(docker inspect -f '{{.State.Pid}}' $container_id 2>/dev/null)
    if [ -z "$pid" ] || [ "$pid" == "0" ]; then
        echo "Error: Container is not running" >&2
        exit 1
    fi
    echo "/proc/$pid/ns/net"
}

# Get short container ID
get_short_id() {
    echo $container_id | cut -c1-12
}

case $action in
    add)
        netns=$(get_netns)
        short_id=$(get_short_id)

        echo "Adding CNI networking to container $short_id..."

        # Create CNI config if not exists
        if [ ! -f "$CNI_CONFIG" ]; then
            sudo mkdir -p $(dirname $CNI_CONFIG)
            sudo tee $CNI_CONFIG > /dev/null << 'EOF'
{
  "cniVersion": "1.0.0",
  "name": "docker-cni",
  "type": "poridhi-cni",
  "bridge": "cni0",
  "ipam": {
    "subnet": "10.244.0.0/24",
    "gateway": "10.244.0.1"
  },
  "dns": {
    "nameservers": ["8.8.8.8", "8.8.4.4"]
  }
}
EOF
        fi

        # Run CNI ADD
        result=$(cat $CNI_CONFIG | \
            CNI_COMMAND=ADD \
            CNI_CONTAINERID=$short_id \
            CNI_NETNS=$netns \
            CNI_IFNAME=eth0 \
            CNI_PATH=$(dirname $CNI_PLUGIN) \
            $CNI_PLUGIN)

        echo "CNI Result:"
        echo $result | jq .

        # Extract and display IP
        ip=$(echo $result | jq -r '.ips[0].address' 2>/dev/null || echo "unknown")
        echo ""
        echo "Container $short_id is now accessible at: $ip"
        ;;

    del)
        netns=$(get_netns)
        short_id=$(get_short_id)

        echo "Removing CNI networking from container $short_id..."

        cat $CNI_CONFIG | \
            CNI_COMMAND=DEL \
            CNI_CONTAINERID=$short_id \
            CNI_NETNS=$netns \
            CNI_IFNAME=eth0 \
            CNI_PATH=$(dirname $CNI_PLUGIN) \
            $CNI_PLUGIN

        echo "Done"
        ;;

    status)
        netns=$(get_netns)
        short_id=$(get_short_id)

        echo "Container: $short_id"
        echo "NetNS: $netns"
        echo ""
        echo "Network interfaces:"
        nsenter --net=$netns ip addr
        echo ""
        echo "Routes:"
        nsenter --net=$netns ip route
        ;;

    *)
        usage
        ;;
esac
