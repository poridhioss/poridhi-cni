#!/bin/bash

# Attach CNI networking to all containers with the "cni-" prefix

set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"

CONTAINERS=$(docker ps --filter "name=cni-" --format "{{.Names}}")

if [ -z "$CONTAINERS" ]; then
    echo "No containers with 'cni-' prefix found."
    echo "Start containers first: sudo docker compose -f ~/docker-compose.yml up -d"
    exit 1
fi

for container in $CONTAINERS; do
    echo "Attaching CNI to $container..."
    $SCRIPT_DIR/docker-cni.sh add $container
    echo ""
done

echo "All containers attached to CNI network!"
echo ""
echo "Container IPs:"
for container in $CONTAINERS; do
    pid=$(docker inspect -f '{{.State.Pid}}' $container)
    ip=$(nsenter --net=/proc/$pid/ns/net ip -4 addr show eth0 2>/dev/null | grep inet | awk '{print $2}')
    echo "  $container: $ip"
done
