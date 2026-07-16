#!/usr/bin/env sh
set -eu

namespace="${NAMESPACE:-tcp-lb-mini}"
pod="tcp-lb-60-second-pod"

echo "==> Removing the previous Lab 1.1 Pod"
kubectl -n "$namespace" delete pod "$pod" --ignore-not-found --wait=true

echo "==> Creating the Pod imperatively"
kubectl -n "$namespace" run "$pod" \
  --image=tcp-lb-mini/pod-state-manager:v2 \
  --image-pull-policy=Never \
  --restart=Never \
  --labels='app=tcp-lb-60-second-pod,component=pod-state-manager,lab=1.1,track=exam-speed' \
  --env='LAB_NAME=the-60-second-pod' \
  --env='REDIS_ADDR=redis:6379' \
  --env='POLL_INTERVAL=60s' \
  --env='MANAGED_SERVICES=tcp-lb-gateway=tcp-lb-gateway:8080,tcp-backend-worker1=tcp-backend-worker1:8080,tcp-backend-worker2=tcp-backend-worker2:8080' \
  --overrides='{"apiVersion":"v1","spec":{"containers":[{"name":"tcp-lb-60-second-pod","resources":{"requests":{"cpu":"25m","memory":"32Mi"},"limits":{"cpu":"100m","memory":"64Mi"}}}]}}' \
  --override-type=strategic

kubectl -n "$namespace" wait --for=condition=Ready "pod/$pod" --timeout=120s
kubectl -n "$namespace" get pod "$pod" --show-labels
kubectl -n "$namespace" get pod "$pod" \
  -o jsonpath='{.spec.containers[0].env}{"\n"}{.spec.containers[0].resources}{"\n"}'
kubectl -n "$namespace" logs "$pod" --tail=10
