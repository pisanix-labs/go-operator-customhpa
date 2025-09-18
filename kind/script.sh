#!/usr/bin/env bash
set -euo pipefail

CLUSTER_NAME="operator-customhpa"
NAMESPACE="default"
KUBECONFIG_FILE="$(pwd)/kubeconfig-kind.yaml"

echo "Tested on Kind v0.29.0, Go 1.24.2, Linux/amd64"
echo
echo "👉 Creating Kind cluster: $CLUSTER_NAME"
kind create cluster --name "$CLUSTER_NAME" --kubeconfig "$KUBECONFIG_FILE" --config=- <<EOF
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
nodes:
  - role: control-plane
  - role: worker
EOF

echo
echo "✅ $CLUSTER_NAME Environment ready!"
echo
