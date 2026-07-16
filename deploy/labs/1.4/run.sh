#!/usr/bin/env sh
set -eu

namespace="${NAMESPACE:-tcp-lb-mini}"
script_dir="$(CDPATH= cd "$(dirname "$0")" && pwd)"
manifest="$script_dir/pods.yaml"

echo "==> Removing previous Lab 1.4 Pods"
kubectl -n "$namespace" delete pods -l lab=1.4 --ignore-not-found --wait=true

echo "==> Bulk-creating five labeled Pods from one YAML file"
kubectl apply -f "$manifest"
kubectl -n "$namespace" wait --for=condition=Ready pod -l lab=1.4 --timeout=120s

echo "==> Equality selector: development Pods"
kubectl -n "$namespace" get pods -l 'lab=1.4,environment=dev' -L environment,track

echo "==> Set-based selector: development or staging"
kubectl -n "$namespace" get pods -l 'lab=1.4,environment in (dev,staging)' -L environment,track

echo "==> Inequality selector: Pods not yet stable"
kubectl -n "$namespace" get pods -l 'lab=1.4,track!=stable' -L environment,track

echo "==> Adding labels and confirming YAML annotations across every Lab 1.4 Pod"
kubectl -n "$namespace" label pods -l lab=1.4 team=platform version=v1 --overwrite
kubectl -n "$namespace" annotate pods -l lab=1.4 \
  lab.tcp-lb/purpose=selector-practice lab.tcp-lb/owner=ckad --overwrite

echo "==> Changing existing labels with --overwrite"
kubectl -n "$namespace" label pods -l lab=1.4 version=v2 --overwrite
kubectl -n "$namespace" label pods -l 'lab=1.4,track=canary' track=stable --overwrite
kubectl -n "$namespace" annotate pods -l lab=1.4 lab.tcp-lb/owner=platform --overwrite

echo "==> Final state"
kubectl -n "$namespace" get pods -l lab=1.4 \
  -L environment,track,team,version \
  -o wide
kubectl -n "$namespace" get pods -l lab=1.4 \
  -o custom-columns='NAME:.metadata.name,PURPOSE:.metadata.annotations.lab\.tcp-lb/purpose,OWNER:.metadata.annotations.lab\.tcp-lb/owner'
