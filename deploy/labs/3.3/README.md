# Lab 3.3 — ServiceAccount & RBAC với pod-state-manager

Lab này cấp cho Pod thật của Deployment `pod-state-manager` quyền tối thiểu để
liệt kê Pod trong namespace `tcp-lb-mini` qua Kubernetes API.

Pod có thêm sidecar `rbac-api-checker`. Sidecar đọc ServiceAccount token và CA do
Kubernetes mount tự động, rồi gọi trực tiếp endpoint:

```text
GET /api/v1/namespaces/tcp-lb-mini/pods
```

## Chạy toàn bộ lab

```bash
sh deploy/labs/3.3/run.sh
```

Script tạo RBAC, patch Deployment, chờ rollout và xác minh API call thực tế. Nó
không hiển thị token ra terminal hoặc log.

## Các tài nguyên RBAC

[rbac.yaml](rbac.yaml) tạo:

- ServiceAccount `pod-state-manager`.
- Role `pod-state-manager-pod-reader` chỉ có verb `list` trên resource `pods`.
- RoleBinding cùng tên liên kết ServiceAccount với Role trong `tcp-lb-mini`.

Role có phạm vi namespace, khác với ClusterRole có thể cấp quyền trên nhiều
namespace hoặc tài nguyên cấp cluster.

```yaml
rules:
  - apiGroups: [""]
    resources: ["pods"]
    verbs: ["list"]
```

Kiểm tra quyền mà không cần vào Pod:

```bash
SA=system:serviceaccount:tcp-lb-mini:pod-state-manager
kubectl auth can-i list pods -n tcp-lb-mini --as "$SA"
kubectl auth can-i delete pods -n tcp-lb-mini --as "$SA"
```

Kết quả mong đợi là `yes` cho `list` và `no` cho `delete`.

## Pod sử dụng ServiceAccount

[deployment-patch.yaml](deployment-patch.yaml) cấu hình:

```yaml
serviceAccountName: pod-state-manager
automountServiceAccountToken: true
```

Sidecar sử dụng các file được mount tự động:

```text
/var/run/secrets/kubernetes.io/serviceaccount/token
/var/run/secrets/kubernetes.io/serviceaccount/ca.crt
```

Lệnh gọi API bên trong sidecar tương đương:

```bash
curl --cacert "$ca_file" \
  -H "Authorization: Bearer $(cat "$token_file")" \
  "https://${KUBERNETES_SERVICE_HOST}:${KUBERNETES_SERVICE_PORT_HTTPS}/api/v1/namespaces/${POD_NAMESPACE}/pods"
```

Sidecar tiếp tục giữ các kiểm soát của Lab 3.2: non-root, root filesystem
read-only, drop toàn bộ capabilities và tắt privilege escalation.

## Xác minh theo cách CKAD

```bash
kubectl -n tcp-lb-mini get sa,role,rolebinding
kubectl -n tcp-lb-mini describe role pod-state-manager-pod-reader

POD=$(kubectl -n tcp-lb-mini get pod -l app=pod-state-manager \
  --sort-by=.metadata.creationTimestamp \
  -o custom-columns=NAME:.metadata.name --no-headers | tail -n 1)

kubectl -n tcp-lb-mini get pod "$POD" \
  -o jsonpath='{.spec.serviceAccountName}{"\n"}'
kubectl -n tcp-lb-mini logs "$POD" -c rbac-api-checker
```

Log mong đợi:

```text
Kubernetes API pod list succeeded namespace=tcp-lb-mini pods=...
```

## Dọn dẹp

Xóa sidecar và ServiceAccount khỏi Pod template trước, rồi xóa RBAC:

```bash
kubectl -n tcp-lb-mini patch deployment/pod-state-manager --type=strategic -p '
spec:
  template:
    metadata:
      labels:
        lab.tcp-lb/rbac: null
    spec:
      serviceAccountName: null
      automountServiceAccountToken: null
      containers:
        - name: rbac-api-checker
          $patch: delete
'
kubectl -n tcp-lb-mini rollout status deployment/pod-state-manager
kubectl delete -f deploy/labs/3.3/rbac.yaml
```
