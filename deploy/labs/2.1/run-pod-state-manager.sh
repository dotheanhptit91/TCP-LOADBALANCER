#!/usr/bin/env sh
set -eu

namespace="${NAMESPACE:-tcp-lb-mini}"
deployment="pod-state-manager"
repository="tcp-lb-mini/pod-state-manager"
source_image="${SOURCE_IMAGE:-$repository:latest}"

echo "==> Preparing pod-state-manager image tags"
for version in v1 v2 bad; do
  docker tag "$source_image" "$repository:$version"
done
kind load docker-image --name kind "$repository:v1" "$repository:v2" "$repository:bad"

echo "==> Configuring a zero-downtime rolling strategy"
kubectl -n "$namespace" patch "deployment/$deployment" --type=merge -p \
  '{"spec":{"minReadySeconds":5,"progressDeadlineSeconds":45,"revisionHistoryLimit":5,"strategy":{"type":"RollingUpdate","rollingUpdate":{"maxSurge":1,"maxUnavailable":0}}}}'

echo "==> Deploying pod-state-manager v1"
kubectl -n "$namespace" patch "deployment/$deployment" --type=strategic -p \
  '{"spec":{"template":{"metadata":{"labels":{"lab.tcp-lb/version":"v1"}},"spec":{"containers":[{"name":"manager","image":"tcp-lb-mini/pod-state-manager:v1","env":[{"name":"LAB_VERSION","value":"v1"},{"name":"POLL_INTERVAL","value":"5s"}]}]}}}}'
kubectl -n "$namespace" rollout status "deployment/$deployment" --timeout=120s
kubectl -n "$namespace" get pods -l app="$deployment" -L lab.tcp-lb/version

echo "==> Rolling pod-state-manager from v1 to v2"
kubectl -n "$namespace" patch "deployment/$deployment" --type=strategic -p \
  '{"spec":{"template":{"metadata":{"labels":{"lab.tcp-lb/version":"v2"}},"spec":{"containers":[{"name":"manager","image":"tcp-lb-mini/pod-state-manager:v2","env":[{"name":"LAB_VERSION","value":"v2"},{"name":"POLL_INTERVAL","value":"5s"}]}]}}}}'
kubectl -n "$namespace" rollout status "deployment/$deployment" --timeout=120s
kubectl -n "$namespace" rollout history "deployment/$deployment"

echo "==> Simulating a bad pod-state-manager deployment"
kubectl -n "$namespace" patch "deployment/$deployment" --type=strategic -p \
  '{"spec":{"template":{"metadata":{"labels":{"lab.tcp-lb/version":"bad"}},"spec":{"containers":[{"name":"manager","image":"tcp-lb-mini/pod-state-manager:bad","env":[{"name":"LAB_VERSION","value":"bad"},{"name":"POLL_INTERVAL","value":"not-a-duration"}]}]}}}}'
if kubectl -n "$namespace" rollout status "deployment/$deployment" --timeout=20s; then
  echo "ERROR: bad rollout unexpectedly succeeded" >&2
  exit 1
fi
kubectl -n "$namespace" get pods -l app="$deployment" -L lab.tcp-lb/version
bad_pod="$(kubectl -n "$namespace" get pods -l app="$deployment",lab.tcp-lb/version=bad -o jsonpath='{.items[0].metadata.name}')"
kubectl -n "$namespace" logs "$bad_pod" || true

echo "==> Rolling back to pod-state-manager v2"
kubectl -n "$namespace" rollout undo "deployment/$deployment"
kubectl -n "$namespace" rollout status "deployment/$deployment" --timeout=120s
kubectl -n "$namespace" get pods -l app="$deployment" -L lab.tcp-lb/version

image="$(kubectl -n "$namespace" get "deployment/$deployment" -o jsonpath='{.spec.template.spec.containers[?(@.name=="manager")].image}')"
version="$(kubectl -n "$namespace" get "deployment/$deployment" -o jsonpath='{.spec.template.metadata.labels.lab\.tcp-lb/version}')"
test "$image" = "$repository:v2"
test "$version" = "v2"

echo "==> Verifying that pod-state-manager still updates state"
sleep 6
kubectl -n "$namespace" logs "deployment/$deployment" --since=15s | grep 'instance state updated'
echo "rollback verified: image=$image version=$version"
