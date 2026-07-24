#!/usr/bin/env sh
set -eu

namespace="${NAMESPACE:-tcp-lb-mini}"
deployment="pod-state-manager"
container="manager"
script_dir="$(CDPATH= cd "$(dirname "$0")" && pwd)"
patch_file="$script_dir/deployment-patch.yaml"

kubectl get namespace "$namespace" >/dev/null
kubectl -n "$namespace" get "deployment/$deployment" >/dev/null

echo "==> Locking down the pod-state-manager Pod template"
kubectl -n "$namespace" patch "deployment/$deployment" \
  --type=strategic --patch-file "$patch_file"
kubectl -n "$namespace" rollout status "deployment/$deployment" --timeout=120s

pod="$(kubectl -n "$namespace" get pods -l app="$deployment" \
  --sort-by=.metadata.creationTimestamp \
  -o custom-columns=NAME:.metadata.name --no-headers | tail -n 1)"
test -n "$pod"

echo "==> Verifying the effective Kubernetes security context"
run_as_non_root="$(kubectl -n "$namespace" get pod "$pod" \
  -o jsonpath='{.spec.securityContext.runAsNonRoot}')"
run_as_user="$(kubectl -n "$namespace" get pod "$pod" \
  -o jsonpath='{.spec.securityContext.runAsUser}')"
read_only_root="$(kubectl -n "$namespace" get pod "$pod" \
  -o jsonpath='{.spec.containers[?(@.name=="manager")].securityContext.readOnlyRootFilesystem}')"
allow_escalation="$(kubectl -n "$namespace" get pod "$pod" \
  -o jsonpath='{.spec.containers[?(@.name=="manager")].securityContext.allowPrivilegeEscalation}')"
dropped_caps="$(kubectl -n "$namespace" get pod "$pod" \
  -o jsonpath='{.spec.containers[?(@.name=="manager")].securityContext.capabilities.drop[*]}')"
seccomp="$(kubectl -n "$namespace" get pod "$pod" \
  -o jsonpath='{.spec.securityContext.seccompProfile.type}')"

test "$run_as_non_root" = true
test "$run_as_user" = 10001
test "$read_only_root" = true
test "$allow_escalation" = false
test "$dropped_caps" = ALL
test "$seccomp" = RuntimeDefault

echo "==> Verifying controls from inside the running container"
runtime_uid="$(kubectl -n "$namespace" exec "$pod" -c "$container" -- id -u)"
runtime_gid="$(kubectl -n "$namespace" exec "$pod" -c "$container" -- id -g)"
test "$runtime_uid" = 10001
test "$runtime_gid" = 10001

if kubectl -n "$namespace" exec "$pod" -c "$container" -- \
  /bin/sh -ec 'touch /tmp/lab-3-2-write-test' >/dev/null 2>&1; then
  echo "ERROR: root filesystem accepted a write" >&2
  exit 1
fi

proc_status="$(kubectl -n "$namespace" exec "$pod" -c "$container" -- \
  /bin/sh -ec 'grep -E "^(CapEff|NoNewPrivs):" /proc/1/status')"
cap_eff="$(printf '%s\n' "$proc_status" | awk '/^CapEff:/ {print $2}')"
no_new_privs="$(printf '%s\n' "$proc_status" | awk '/^NoNewPrivs:/ {print $2}')"
test "$cap_eff" = 0000000000000000
test "$no_new_privs" = 1

kubectl -n "$namespace" get pod "$pod" -o wide
printf 'runAsNonRoot=%s uid=%s gid=%s readOnlyRootFilesystem=%s\n' \
  "$run_as_non_root" "$runtime_uid" "$runtime_gid" "$read_only_root"
printf 'allowPrivilegeEscalation=%s droppedCapabilities=%s seccomp=%s\n' \
  "$allow_escalation" "$dropped_caps" "$seccomp"
printf 'CapEff=%s NoNewPrivs=%s rootFilesystemWrite=blocked\n' \
  "$cap_eff" "$no_new_privs"

echo "==> Verifying pod-state-manager still updates service state"
kubectl -n "$namespace" logs "$pod" -c "$container" --since=30s | \
  grep 'instance state updated' | tail -n 3
