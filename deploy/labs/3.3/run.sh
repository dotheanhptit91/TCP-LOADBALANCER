#!/usr/bin/env sh
set -eu

namespace="${NAMESPACE:-tcp-lb-mini}"
deployment="pod-state-manager"
service_account="pod-state-manager"
checker_image="${RBAC_CHECKER_IMAGE:-curlimages/curl:8.12.1}"
kind_cluster="${KIND_CLUSTER_NAME:-kind}"
script_dir="$(CDPATH= cd "$(dirname "$0")" && pwd)"
rbac_file="$script_dir/rbac.yaml"
patch_file="$script_dir/deployment-patch.yaml"

kubectl get namespace "$namespace" >/dev/null
kubectl -n "$namespace" get "deployment/$deployment" >/dev/null

if command -v docker >/dev/null 2>&1 && command -v kind >/dev/null 2>&1 && \
   docker image inspect "$checker_image" >/dev/null 2>&1 && \
   kind get clusters | grep -qx "$kind_cluster"; then
  echo "==> Loading the API checker image into Kind"
  kind load docker-image --name "$kind_cluster" "$checker_image"
fi

echo "==> Creating ServiceAccount, Role, and RoleBinding"
kubectl apply -f "$rbac_file"

echo "==> Verifying least-privilege RBAC rules"
subject="system:serviceaccount:${namespace}:${service_account}"
test "$(kubectl auth can-i list pods -n "$namespace" --as "$subject")" = yes
test "$(kubectl auth can-i delete pods -n "$namespace" --as "$subject")" = no

echo "==> Assigning the ServiceAccount and API checker to pod-state-manager"
kubectl -n "$namespace" patch "deployment/$deployment" \
  --type=strategic --patch-file "$patch_file"
kubectl -n "$namespace" rollout status "deployment/$deployment" --timeout=180s

pod="$(kubectl -n "$namespace" get pods -l app="$deployment" \
  --sort-by=.metadata.creationTimestamp \
  -o custom-columns=NAME:.metadata.name --no-headers | tail -n 1)"
test -n "$pod"

echo "==> Verifying ServiceAccount token and direct Kubernetes API access"
test "$(kubectl -n "$namespace" get pod "$pod" \
  -o jsonpath='{.spec.serviceAccountName}')" = "$service_account"
kubectl -n "$namespace" exec "$pod" -c rbac-api-checker -- /bin/sh -ec '
  token_file=/var/run/secrets/kubernetes.io/serviceaccount/token
  ca_file=/var/run/secrets/kubernetes.io/serviceaccount/ca.crt
  test -s "$token_file"
  test -s "$ca_file"
  api="https://${KUBERNETES_SERVICE_HOST}:${KUBERNETES_SERVICE_PORT_HTTPS}/api/v1/namespaces/${POD_NAMESPACE}/pods"
  response="$(curl --fail --silent --show-error --cacert "$ca_file" \
    -H "Authorization: Bearer $(cat "$token_file")" "$api")"
  printf "%s\n" "$response" | \
    grep -Eq "\"kind\"[[:space:]]*:[[:space:]]*\"PodList\""
'

kubectl -n "$namespace" get serviceaccount "$service_account"
kubectl -n "$namespace" get role,rolebinding \
  pod-state-manager-pod-reader
kubectl -n "$namespace" get pod "$pod" -o wide
kubectl -n "$namespace" logs "$pod" -c rbac-api-checker --tail=5

echo "==> Verifying pod-state-manager remains healthy"
kubectl -n "$namespace" logs "$pod" -c manager --since=30s | \
  grep 'instance state updated' | tail -n 3
