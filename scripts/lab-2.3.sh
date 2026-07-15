#!/usr/bin/env sh
set -eu

namespace="${NAMESPACE:-tcp-lb-mini}"
deployment="pod-state-manager-hpa"

if ! kubectl top nodes >/dev/null 2>&1; then
  echo "==> Installing Metrics Server v0.8.1"
  kubectl apply -f https://github.com/kubernetes-sigs/metrics-server/releases/download/v0.8.1/components.yaml
  if ! kubectl -n kube-system get deployment metrics-server -o jsonpath='{.spec.template.spec.containers[0].args}' | grep -q -- '--kubelet-insecure-tls'; then
    kubectl -n kube-system patch deployment metrics-server --type=json -p \
      '[{"op":"add","path":"/spec/template/spec/containers/0/args/-","value":"--kubelet-insecure-tls"}]'
  fi
  kubectl -n kube-system rollout status deployment/metrics-server --timeout=180s
  i=0
  until kubectl top nodes >/dev/null 2>&1; do
    i=$((i + 1))
    [ "$i" -lt 24 ] || { echo "Metrics API did not become ready" >&2; exit 1; }
    sleep 5
  done
fi

echo "==> Manually scaling pod-state-manager to 10 replicas"
kubectl apply -f deploy/labs/2.3-scale-deployment.yaml
kubectl -n "$namespace" scale "deployment/$deployment" --replicas=10
kubectl -n "$namespace" rollout status "deployment/$deployment" --timeout=180s
kubectl -n "$namespace" get "deployment/$deployment"
test "$(kubectl -n "$namespace" get "deployment/$deployment" -o jsonpath='{.status.readyReplicas}')" = "10"

echo "==> Configuring HPA with a 50% CPU target"
kubectl apply -f deploy/labs/2.3-hpa.yaml
i=0
until [ -n "$(kubectl -n "$namespace" get hpa "$deployment" -o jsonpath='{.status.currentMetrics[0].resource.current.averageUtilization}' 2>/dev/null)" ]; do
  i=$((i + 1))
  [ "$i" -lt 24 ] || { echo "HPA did not receive CPU metrics" >&2; exit 1; }
  sleep 5
done
kubectl -n "$namespace" get hpa "$deployment"
kubectl -n "$namespace" top pods -l app="$deployment"
