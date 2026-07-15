#!/usr/bin/env sh
set -eu

namespace="${NAMESPACE:-tcp-lb-mini}"
service="pod-state-manager-active"

kubectl apply -f deploy/labs/2.2-blue-green.yaml
kubectl -n "$namespace" rollout status deployment/pod-state-manager-blue --timeout=120s
kubectl -n "$namespace" rollout status deployment/pod-state-manager-green --timeout=120s

echo "==> Service initially routes to blue"
blue="$(kubectl -n "$namespace" exec deployment/redis -- wget -qO- "http://$service:8080")"
echo "$blue"
echo "$blue" | grep -q 'color=blue version=v1'

echo "==> Flipping the single Service selector to green"
kubectl -n "$namespace" patch "service/$service" --type=merge -p \
  '{"spec":{"selector":{"app":"pod-state-manager-bg","slot":"green"}}}'
green=""
for _ in $(seq 1 15); do
  green="$(kubectl -n "$namespace" exec deployment/redis -- wget -qO- "http://$service:8080")"
  echo "$green" | grep -q 'color=green version=v2' && break
  sleep 2
done
echo "$green"
echo "$green" | grep -q 'color=green version=v2'

kubectl -n "$namespace" get deployments pod-state-manager-blue pod-state-manager-green
kubectl -n "$namespace" get service "$service" -o wide
kubectl -n "$namespace" get endpointslice -l "kubernetes.io/service-name=$service"
