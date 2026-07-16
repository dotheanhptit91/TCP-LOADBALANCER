#!/usr/bin/env sh
set -eu

namespace="${NAMESPACE:-tcp-lb-mini}"
deployment="pod-state-manager-kustomize"
script_dir="$(CDPATH= cd "$(dirname "$0")" && pwd)"
overlay="$script_dir/kustomize/overlays/lab"

echo "==> Rendering overlay"
rendered="$(kubectl kustomize "$overlay")"
echo "$rendered" | grep -q 'replicas: 3'
echo "$rendered" | grep -q 'image: tcp-lb-mini/pod-state-manager:v2'

echo "==> Applying overlay"
kubectl apply -k "$overlay"
kubectl -n "$namespace" rollout status "deployment/$deployment" --timeout=120s
kubectl -n "$namespace" get "deployment/$deployment" -o \
  custom-columns='NAME:.metadata.name,REPLICAS:.spec.replicas,IMAGE:.spec.template.spec.containers[0].image'

test "$(kubectl -n "$namespace" get "deployment/$deployment" -o jsonpath='{.spec.replicas}')" = "3"
test "$(kubectl -n "$namespace" get "deployment/$deployment" -o jsonpath='{.spec.template.spec.containers[0].image}')" = "tcp-lb-mini/pod-state-manager:v2"
