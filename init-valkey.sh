#!/bin/sh
set -e

NODES="valkey-0 valkey-1 valkey-2 valkey-3 valkey-4 valkey-5"

echo "Waiting for Valkey nodes..."

for node in $NODES; do
  until valkey-cli -h "$node" -p 6379 -a "$VALKEY_PASSWORD" PING >/dev/null 2>&1; do
    sleep 1
  done
done

metadata_exists=false

for i in $(seq 1 60); do
  if valkey-cli -h valkey-0 -p 6379 -a "$VALKEY_PASSWORD" CLUSTER INFO 2>/dev/null | grep -q "cluster_state:ok"; then
    echo "Valkey cluster already initialized"
    exit 0
  fi

  if valkey-cli -h valkey-0 -p 6379 -a "$VALKEY_PASSWORD" CLUSTER NODES 2>/dev/null | grep -qv myself; then
    metadata_exists=true
    echo "Valkey cluster metadata exists, waiting for cluster_state:ok..."
    sleep 1
    continue
  fi

  break
done

if [ "$metadata_exists" = true ]; then
  echo "Valkey cluster metadata exists, but cluster_state did not become ok"
  valkey-cli -h valkey-0 -p 6379 -a "$VALKEY_PASSWORD" CLUSTER INFO || true
  valkey-cli -h valkey-0 -p 6379 -a "$VALKEY_PASSWORD" CLUSTER NODES || true
  exit 1
fi

echo "Creating Valkey cluster..."

valkey-cli --cluster create \
  valkey-0:6379 valkey-1:6379 valkey-2:6379 \
  valkey-3:6379 valkey-4:6379 valkey-5:6379 \
  -a "$VALKEY_PASSWORD" \
  --cluster-replicas 1 \
  --cluster-yes
