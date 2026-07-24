# Lab 3.2 — Security Context Lockdown với pod-state-manager

Lab này harden Pod template của Deployment thật `pod-state-manager`. Kubernetes
tạo Pod mới chạy non-root, không được ghi vào root filesystem, không có Linux
capability và không được privilege escalation.

Không sửa trực tiếp Pod `pod-state-manager-...`, vì Deployment sẽ thay thế và làm
mất mọi thay đổi trực tiếp trên Pod.

## Chạy toàn bộ lab

```bash
sh deploy/labs/3.2/run.sh
```

Script có thể chạy từ bất kỳ thư mục nào. Nó patch Deployment, chờ rollout và
xác minh chính sách từ Kubernetes API lẫn bên trong container.

## Security context được áp dụng

[deployment-patch.yaml](deployment-patch.yaml) cấu hình ở cấp Pod:

```yaml
securityContext:
  runAsNonRoot: true
  runAsUser: 10001
  runAsGroup: 10001
  seccompProfile:
    type: RuntimeDefault
```

Và ở cấp container `manager`:

```yaml
securityContext:
  allowPrivilegeEscalation: false
  readOnlyRootFilesystem: true
  capabilities:
    drop:
      - ALL
```

Ý nghĩa:

- `runAsNonRoot`: kubelet từ chối chạy container với UID 0.
- `runAsUser` và `runAsGroup`: đặt UID/GID cố định là `10001`.
- `readOnlyRootFilesystem`: chặn ghi vào filesystem của image.
- `drop: ALL`: loại bỏ toàn bộ Linux capabilities mặc định.
- `allowPrivilegeEscalation: false`: bật `no_new_privs`, ngăn tiến trình con nhận
  quyền cao hơn qua setuid/setgid hoặc cơ chế tương tự.
- `RuntimeDefault`: sử dụng seccomp profile mặc định của container runtime.

## Áp dụng thủ công

```bash
kubectl -n tcp-lb-mini patch deployment/pod-state-manager \
  --type=strategic \
  --patch-file deploy/labs/3.2/deployment-patch.yaml
kubectl -n tcp-lb-mini rollout status deployment/pod-state-manager
```

## Xác minh theo cách CKAD

```bash
POD=$(kubectl -n tcp-lb-mini get pod -l app=pod-state-manager \
  --sort-by=.metadata.creationTimestamp \
  -o custom-columns=NAME:.metadata.name --no-headers | tail -n 1)

kubectl -n tcp-lb-mini get pod "$POD" \
  -o jsonpath='{.spec.securityContext}{"\n"}{.spec.containers[?(@.name=="manager")].securityContext}{"\n"}'

kubectl -n tcp-lb-mini exec "$POD" -c manager -- id
kubectl -n tcp-lb-mini exec "$POD" -c manager -- \
  grep -E '^(CapEff|NoNewPrivs):' /proc/1/status
```

Kết quả mong đợi:

```text
uid=10001 gid=10001
CapEff:  0000000000000000
NoNewPrivs:  1
```

Thử ghi vào root filesystem phải thất bại:

```bash
kubectl -n tcp-lb-mini exec "$POD" -c manager -- touch /tmp/test
```

## Khi ứng dụng cần ghi file

Không tắt `readOnlyRootFilesystem`. Hãy mount một `emptyDir`, PVC hoặc volume phù
hợp đúng tại đường dẫn ứng dụng cần ghi. Chỉ volume đó có thể ghi; filesystem từ
image vẫn read-only.

## Dọn dẹp security context của lab

```bash
kubectl -n tcp-lb-mini patch deployment/pod-state-manager --type=strategic -p '
spec:
  template:
    metadata:
      labels:
        lab.tcp-lb/security-lockdown: null
    spec:
      securityContext: null
      containers:
        - name: manager
          securityContext: null
'
kubectl -n tcp-lb-mini rollout status deployment/pod-state-manager
```
