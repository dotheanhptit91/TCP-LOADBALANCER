#!/usr/bin/env sh
set -eu

namespace="${NAMESPACE:-tcp-lb-mini}"
deployment="pod-state-manager"
secret="tcp-lb-lab-3-1-secret"
config_map="tcp-lb-lab-3-1-config"
script_dir="$(CDPATH= cd "$(dirname "$0")" && pwd)"
secret_file="$script_dir/files/api-token.txt"
patch_file="$script_dir/deployment-patch.yaml"

kubectl get namespace "$namespace" >/dev/null
kubectl -n "$namespace" get "deployment/$deployment" >/dev/null
test -s "$secret_file"

echo "==> Creating Secret from file"
kubectl -n "$namespace" create secret generic "$secret" \
  --from-file="api-token=$secret_file" \
  --dry-run=client -o yaml | kubectl apply -f -

echo "==> Creating ConfigMap from literals"
kubectl -n "$namespace" create configmap "$config_map" \
  --from-literal=app-mode=lab \
  --from-literal=log-level=debug \
  --dry-run=client -o yaml | kubectl apply -f -

echo "==> Injecting Secret and ConfigMap into pod-state-manager"
kubectl -n "$namespace" patch "deployment/$deployment" \
  --type=strategic --patch-file "$patch_file"

# Secret-backed environment variables are evaluated only when a container starts.
# Restarting also makes repeated lab runs consume the current Secret file.
kubectl -n "$namespace" rollout restart "deployment/$deployment"
kubectl -n "$namespace" rollout status "deployment/$deployment" --timeout=120s

pod="$(kubectl -n "$namespace" get pods -l app="$deployment" \
  --sort-by=.metadata.creationTimestamp \
  -o custom-columns=NAME:.metadata.name --no-headers | tail -n 1)"
test -n "$pod"

echo "==> Verifying Secret environment variable without revealing its value"
expected_bytes="$(wc -c < "$secret_file" | tr -d ' ')"
actual_bytes="$(kubectl -n "$namespace" exec "$pod" -c manager -- \
  /bin/sh -ec 'printf %s "$BACKEND_API_TOKEN" | wc -c' | tr -d ' ')"
test "$actual_bytes" = "$expected_bytes"
printf 'Secret verified: pod=%s key=api-token bytes=%s\n' "$pod" "$actual_bytes"

echo "==> Verifying ConfigMap volume"
kubectl -n "$namespace" exec "$pod" -c manager -- /bin/sh -ec \
  'test "$(cat /etc/tcp-lb-config/app-mode)" = lab &&
   test "$(cat /etc/tcp-lb-config/log-level)" = debug'

kubectl -n "$namespace" get secret "$secret"
kubectl -n "$namespace" get configmap "$config_map"
kubectl -n "$namespace" get pod "$pod" -o wide
kubectl -n "$namespace" get "deployment/$deployment" -o jsonpath='secretEnv={.spec.template.spec.containers[?(@.name=="manager")].env[?(@.name=="BACKEND_API_TOKEN")].valueFrom.secretKeyRef.name}/{.spec.template.spec.containers[?(@.name=="manager")].env[?(@.name=="BACKEND_API_TOKEN")].valueFrom.secretKeyRef.key}{"\n"}configMapVolume={.spec.template.spec.volumes[?(@.name=="lab-3-1-config")].configMap.name}{"\n"}'
