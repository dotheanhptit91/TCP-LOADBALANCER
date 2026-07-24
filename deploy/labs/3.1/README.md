# Lab 3.1 — ConfigMap & Secret Injection với pod-state-manager

Lab này tạo Secret từ file và ConfigMap từ literal, sau đó patch Deployment thật
`pod-state-manager`. Pod mới do Deployment tạo ra nhận Secret qua biến môi trường
và ConfigMap qua volume read-only.

Không sửa trực tiếp Pod có tên dạng `pod-state-manager-...`, vì Pod đó thuộc quyền
quản lý của Deployment và mọi thay đổi trực tiếp sẽ mất khi Pod được thay thế.

## Chạy toàn bộ lab

```bash
sh deploy/labs/3.1/run.sh
```

Script tự xác định vị trí file nên có thể chạy từ bất kỳ thư mục nào. Nó thực hiện:

1. Tạo Secret `tcp-lb-lab-3-1-secret` từ `files/api-token.txt`.
2. Tạo ConfigMap `tcp-lb-lab-3-1-config` từ hai literal.
3. Strategic-merge patch Pod template của Deployment `pod-state-manager`.
4. Restart rollout để Pod mới luôn đọc phiên bản Secret hiện tại.
5. Xác minh Secret env và các file ConfigMap mà không in nội dung Secret.

File token trong lab là dữ liệu giả. Không commit thông tin xác thực thật vào Git.

## 1. Tạo Secret từ file

```bash
kubectl -n tcp-lb-mini create secret generic tcp-lb-lab-3-1-secret \
  --from-file=api-token=deploy/labs/3.1/files/api-token.txt \
  --dry-run=client -o yaml | kubectl apply -f -
```

Key `api-token` được inject thành biến `BACKEND_API_TOKEN`. Dữ liệu trong trường
`.data` của Secret chỉ được mã hóa base64, không phải mã hóa bảo mật.

## 2. Tạo ConfigMap từ literal

```bash
kubectl -n tcp-lb-mini create configmap tcp-lb-lab-3-1-config \
  --from-literal=app-mode=lab \
  --from-literal=log-level=debug \
  --dry-run=client -o yaml | kubectl apply -f -
```

## 3. Inject vào Pod template của pod-state-manager

Patch [deployment-patch.yaml](deployment-patch.yaml) thêm vào container `manager`:

```yaml
env:
  - name: BACKEND_API_TOKEN
    valueFrom:
      secretKeyRef:
        name: tcp-lb-lab-3-1-secret
        key: api-token
volumeMounts:
  - name: lab-3-1-config
    mountPath: /etc/tcp-lb-config
    readOnly: true
```

Volume lấy dữ liệu từ ConfigMap:

```yaml
volumes:
  - name: lab-3-1-config
    configMap:
      name: tcp-lb-lab-3-1-config
```

Áp dụng thủ công:

```bash
kubectl -n tcp-lb-mini patch deployment/pod-state-manager \
  --type=strategic \
  --patch-file deploy/labs/3.1/deployment-patch.yaml
kubectl -n tcp-lb-mini rollout restart deployment/pod-state-manager
kubectl -n tcp-lb-mini rollout status deployment/pod-state-manager
```

## 4. Xác minh

```bash
POD=$(kubectl -n tcp-lb-mini get pod -l app=pod-state-manager \
  --sort-by=.metadata.creationTimestamp \
  -o custom-columns=NAME:.metadata.name --no-headers | tail -n 1)

# Không hiển thị giá trị Secret.
kubectl -n tcp-lb-mini exec "$POD" -c manager -- \
  /bin/sh -ec 'test -n "$BACKEND_API_TOKEN" && echo secret-injected'

kubectl -n tcp-lb-mini exec "$POD" -c manager -- \
  ls -l /etc/tcp-lb-config
kubectl -n tcp-lb-mini exec "$POD" -c manager -- \
  cat /etc/tcp-lb-config/app-mode /etc/tcp-lb-config/log-level
```

Secret qua environment chỉ được đọc khi container khởi động. ConfigMap mount qua
volume được kubelet cập nhật dần, nhưng ứng dụng phải đọc lại file để nhận thay đổi.

## Dọn dẹp phần injection

Xóa các trường đã thêm khỏi Pod template rồi xóa Secret và ConfigMap:

```bash
kubectl -n tcp-lb-mini patch deployment/pod-state-manager --type=strategic -p '
spec:
  template:
    metadata:
      labels:
        lab.tcp-lb/config-injection: null
    spec:
      containers:
        - name: manager
          env:
            - name: BACKEND_API_TOKEN
              $patch: delete
          volumeMounts:
            - mountPath: /etc/tcp-lb-config
              $patch: delete
      volumes:
        - name: lab-3-1-config
          $patch: delete
'
kubectl -n tcp-lb-mini rollout status deployment/pod-state-manager
kubectl -n tcp-lb-mini delete secret tcp-lb-lab-3-1-secret
kubectl -n tcp-lb-mini delete configmap tcp-lb-lab-3-1-config
```
