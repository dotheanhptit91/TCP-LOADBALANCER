#!/usr/bin/env sh
set -eu

namespace="${NAMESPACE:-tcp-lb-mini}"
deployment="pod-state-manager"
quota="tcp-lb-namespace-quota"
limit_range="tcp-lb-container-limits"
rejected_pod="pod-state-manager-quota-rejected"
script_dir="$(CDPATH= cd "$(dirname "$0")" && pwd)"
quota_file="$script_dir/quota.yaml"
rejected_pod_file="$script_dir/rejected-pod.yaml"

kubectl get namespace "$namespace" >/dev/null
kubectl -n "$namespace" get "deployment/$deployment" >/dev/null

echo "==> Applying LimitRange and ResourceQuota"
kubectl apply -f "$quota_file"

i=0
until [ "$(kubectl -n "$namespace" get resourcequota "$quota" \
  -o jsonpath='{.status.hard.requests\.cpu}' 2>/dev/null)" = 4 ]; do
  i=$((i + 1))
  [ "$i" -lt 30 ] || { echo "ResourceQuota status was not reconciled" >&2; exit 1; }
  sleep 1
done

echo "==> Recreating the real pod-state-manager Pod under LimitRange admission"
kubectl -n "$namespace" rollout restart "deployment/$deployment"
kubectl -n "$namespace" rollout status "deployment/$deployment" --timeout=180s

pod="$(kubectl -n "$namespace" get pods -l app="$deployment" \
  --sort-by=.metadata.creationTimestamp \
  -o custom-columns=NAME:.metadata.name --no-headers | tail -n 1)"
test -n "$pod"

manager_request_cpu="$(kubectl -n "$namespace" get pod "$pod" \
  -o jsonpath='{.spec.containers[?(@.name=="manager")].resources.requests.cpu}')"
manager_request_memory="$(kubectl -n "$namespace" get pod "$pod" \
  -o jsonpath='{.spec.containers[?(@.name=="manager")].resources.requests.memory}')"
manager_limit_cpu="$(kubectl -n "$namespace" get pod "$pod" \
  -o jsonpath='{.spec.containers[?(@.name=="manager")].resources.limits.cpu}')"
manager_limit_memory="$(kubectl -n "$namespace" get pod "$pod" \
  -o jsonpath='{.spec.containers[?(@.name=="manager")].resources.limits.memory}')"

test "$manager_request_cpu" = 25m
test "$manager_request_memory" = 32Mi
test "$manager_limit_cpu" = 100m
test "$manager_limit_memory" = 128Mi

echo "==> Attempting to create a Pod whose 5 CPU request exceeds the 4 CPU quota"
kubectl -n "$namespace" delete pod "$rejected_pod" --ignore-not-found --wait=true
if rejection="$(kubectl create -f "$rejected_pod_file" 2>&1)"; then
  kubectl -n "$namespace" delete pod "$rejected_pod" --ignore-not-found --wait=false
  echo "ERROR: over-quota Pod was unexpectedly accepted" >&2
  exit 1
fi
printf '%s\n' "$rejection"
printf '%s\n' "$rejection" | grep -q 'exceeded quota'
printf '%s\n' "$rejection" | grep -q 'requests.cpu'
if kubectl -n "$namespace" get pod "$rejected_pod" >/dev/null 2>&1; then
  echo "ERROR: rejected Pod object exists" >&2
  exit 1
fi

echo "==> Final quota and real Pod state"
kubectl -n "$namespace" get resourcequota "$quota"
kubectl -n "$namespace" get limitrange "$limit_range"
kubectl -n "$namespace" get pod "$pod" -o wide
printf 'LimitRange defaults on manager: requests=%s/%s limits=%s/%s\n' \
  "$manager_request_cpu" "$manager_request_memory" \
  "$manager_limit_cpu" "$manager_limit_memory"
printf 'Quota rejection verified: requested CPU=5 quota CPU=4\n'

kubectl -n "$namespace" logs "$pod" -c manager --since=30s | \
  grep 'instance state updated' | tail -n 3
