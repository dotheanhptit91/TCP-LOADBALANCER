#!/usr/bin/env sh
set -eu

namespace="${NAMESPACE:-tcp-lb-mini}"
pod="tcp-lb-init-sidecar"
manifest="deploy/labs/1.2/pod.yaml"

echo "==> Removing the previous Lab 1.2 Pod"
kubectl -n "$namespace" delete pod "$pod" --ignore-not-found --wait=true

echo "==> Applying the init container, app, and sidecar Pod"
kubectl apply -f "$manifest"
kubectl -n "$namespace" wait --for=condition=Ready "pod/$pod" --timeout=120s

kubectl -n "$namespace" get pod "$pod" \
  -o jsonpath='{.status.initContainerStatuses[*].state.terminated.reason}{"\n"}{.status.containerStatuses[*].ready}{"\n"}'
kubectl -n "$namespace" logs "$pod" -c init-config
kubectl -n "$namespace" logs "$pod" -c log-sidecar --tail=20
