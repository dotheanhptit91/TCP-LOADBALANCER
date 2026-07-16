#!/usr/bin/env sh
set -eu

namespace="${NAMESPACE:-tcp-lb-mini}"
deployment="tcp-lb-rollout-demo"
source_image="${SOURCE_IMAGE:-tcp-lb-mini/tcp-backend-worker:latest}"
script_dir="$(CDPATH= cd "$(dirname "$0")" && pwd)"
manifest="$script_dir/rolling-update.yaml"

echo "==> Preparing local v1, v2, and bad image tags"
for version in v1 v2 bad; do
  docker tag "$source_image" "tcp-lb-mini/tcp-backend-worker:$version"
done
kind load docker-image --name kind \
  tcp-lb-mini/tcp-backend-worker:v1 \
  tcp-lb-mini/tcp-backend-worker:v2 \
  tcp-lb-mini/tcp-backend-worker:bad

echo "==> Deploying v1"
kubectl apply -f "$manifest"
kubectl -n "$namespace" rollout status "deployment/$deployment" --timeout=120s
kubectl -n "$namespace" get pods -l app="$deployment" -L lab.tcp-lb/version

echo "==> Rolling from v1 to v2"
kubectl -n "$namespace" patch "deployment/$deployment" --type=strategic -p \
  '{"spec":{"template":{"metadata":{"annotations":{"lab.tcp-lb/version":"v2"},"labels":{"lab.tcp-lb/version":"v2"}},"spec":{"containers":[{"name":"demo","image":"tcp-lb-mini/tcp-backend-worker:v2","env":[{"name":"WORKER_NAME","value":"rollout-demo-v2"},{"name":"SOURCE_PORT_RANGE","value":"3000-3010"}]}]}}}}'
kubectl -n "$namespace" rollout status "deployment/$deployment" --timeout=120s
kubectl -n "$namespace" rollout history "deployment/$deployment"

echo "==> Simulating a bad deployment"
kubectl -n "$namespace" patch "deployment/$deployment" --type=strategic -p \
  '{"spec":{"template":{"metadata":{"annotations":{"lab.tcp-lb/version":"bad"},"labels":{"lab.tcp-lb/version":"bad"}},"spec":{"containers":[{"name":"demo","image":"tcp-lb-mini/tcp-backend-worker:bad","env":[{"name":"WORKER_NAME","value":"rollout-demo-bad"},{"name":"SOURCE_PORT_RANGE","value":"invalid"}]}]}}}}'
if kubectl -n "$namespace" rollout status "deployment/$deployment" --timeout=20s; then
  echo "ERROR: bad rollout unexpectedly succeeded" >&2
  exit 1
fi
kubectl -n "$namespace" get pods -l app="$deployment"
bad_pod="$(kubectl -n "$namespace" get pods -l app="$deployment",lab.tcp-lb/version=bad -o jsonpath='{.items[0].metadata.name}')"
kubectl -n "$namespace" logs "$bad_pod" || true

echo "==> Rolling back to the previous revision"
kubectl -n "$namespace" rollout undo "deployment/$deployment"
kubectl -n "$namespace" rollout status "deployment/$deployment" --timeout=120s
kubectl -n "$namespace" rollout history "deployment/$deployment"
kubectl -n "$namespace" get pods -l app="$deployment"

echo "==> Verifying the service after rollback"
response="$(kubectl -n "$namespace" exec deployment/redis -- wget -qO- "http://$deployment:8080/healthz")"
echo "$response"
echo "$response" | grep -q '"service":"rollout-demo-v2"'
