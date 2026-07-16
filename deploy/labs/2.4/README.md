# Bài thực hành 2.4 — Quản lý cấu hình bằng Kustomize

## Mục tiêu

Bài thực hành sử dụng Kustomize để tạo một cấu hình dùng chung và một lớp phủ
dành cho môi trường lab. Lớp phủ thay đổi phiên bản image, số replica và biến
môi trường mà không sao chép toàn bộ manifest Deployment.

Sau khi hoàn thành, bạn sẽ biết cách:

- Tổ chức manifest theo mô hình `base` và `overlays`.
- Thay thẻ image bằng trường `images`.
- Thay số replica bằng trường `replicas`.
- Áp dụng JSON patch cho một trường cụ thể.
- Xem manifest sau khi kết xuất trước khi triển khai.

## Cấu trúc thư mục

```text
deploy/labs/2.4/kustomize/
├── base/
│   ├── deployment.yaml
│   └── kustomization.yaml
└── overlays/
    └── lab/
        └── kustomization.yaml
```

Kustomize không sửa trực tiếp các tệp trong `base`. Nó đọc tài nguyên cơ sở rồi
áp dụng lần lượt các phép biến đổi được khai báo trong lớp phủ.

## Cấu hình cơ sở và lớp phủ

Base định nghĩa Deployment `pod-state-manager-kustomize` với:

- Image `tcp-lb-mini/pod-state-manager:v1`.
- Một replica.
- `LAB_VERSION=kustomize-base-v1`.
- Namespace `tcp-lb-mini` được khai báo trong `base/kustomization.yaml`.

Lớp phủ `overlays/lab` tạo ra ba thay đổi:

| Trường | Giá trị base | Giá trị sau lớp phủ lab |
|---|---|---|
| Image | `tcp-lb-mini/pod-state-manager:v1` | `tcp-lb-mini/pod-state-manager:v2` |
| Replica | `1` | `3` |
| `LAB_VERSION` | `kustomize-base-v1` | `kustomize-overlay-v2` |

Thẻ image và số replica dùng các transformer tích hợp của Kustomize. Giá trị
`LAB_VERSION` được thay bằng JSON patch nhắm đúng Deployment và đường dẫn:
`/spec/template/spec/containers/0/env/0/value`.

## Điều kiện tiên quyết

- Cluster Kubernetes và namespace `tcp-lb-mini` đang hoạt động.
- `kubectl` hỗ trợ các lệnh `kubectl kustomize` và `kubectl apply -k`.
- Image `tcp-lb-mini/pod-state-manager:v2` đã có trên node vì manifest dùng
  `imagePullPolicy: Never`.
- Redis và các dịch vụ do `pod-state-manager` theo dõi đã tồn tại nếu muốn kiểm
  tra đầy đủ hoạt động của ứng dụng.

Kiểm tra phiên bản công cụ và namespace:

```bash
kubectl version --client
kubectl get namespace tcp-lb-mini
```

## Chạy tự động

Từ thư mục gốc của dự án:

```bash
sh deploy/labs/2.4/run.sh
```

Kịch bản sẽ:

1. Kết xuất lớp phủ bằng `kubectl kustomize` mà chưa thay đổi cluster.
2. Xác minh manifest kết xuất có `replicas: 3`.
3. Xác minh image đã trở thành `tcp-lb-mini/pod-state-manager:v2`.
4. Áp dụng lớp phủ bằng `kubectl apply -k`.
5. Chờ Deployment hoàn tất rollout.
6. Đọc lại Deployment và xác minh chính xác số replica cùng image đang chạy.

## Kết xuất và kiểm tra manifest

Đường dẫn lớp phủ:

```bash
OVERLAY=deploy/labs/2.4/kustomize/overlays/lab
```

Xem kết quả cuối cùng mà không áp dụng vào cluster:

```bash
kubectl kustomize "$OVERLAY"
```

Có thể lọc các trường quan trọng:

```bash
kubectl kustomize "$OVERLAY" | \
  grep -E 'name: pod-state-manager-kustomize|replicas:|image:|kustomize-overlay-v2'
```

Bước kết xuất giúp phát hiện sai đường dẫn patch, sai tên image hoặc giá trị
replica trước khi cấu hình ảnh hưởng tới cluster.

## Áp dụng lớp phủ

```bash
kubectl apply -k "$OVERLAY"
kubectl -n tcp-lb-mini rollout status \
  deployment/pod-state-manager-kustomize --timeout=120s
```

Kiểm tra kết quả:

```bash
kubectl -n tcp-lb-mini get deployment/pod-state-manager-kustomize -o \
  custom-columns='NAME:.metadata.name,REPLICAS:.spec.replicas,READY:.status.readyReplicas,IMAGE:.spec.template.spec.containers[0].image'

kubectl -n tcp-lb-mini get deployment/pod-state-manager-kustomize \
  -o jsonpath='{.spec.template.spec.containers[0].env[?(@.name=="LAB_VERSION")].value}'; echo
```

Kết quả mong đợi:

- `REPLICAS` và `READY` đều bằng `3`.
- `IMAGE` là `tcp-lb-mini/pod-state-manager:v2`.
- `LAB_VERSION` là `kustomize-overlay-v2`.

## Thử thay đổi lớp phủ

Để thực hành thêm, có thể sửa `count`, `newTag` hoặc giá trị JSON patch trong
`overlays/lab/kustomization.yaml`, sau đó kết xuất lại để quan sát sự khác biệt.
Mỗi lần thay đổi nên chạy `kubectl kustomize` trước rồi mới chạy `kubectl apply -k`.

Không sửa `base/deployment.yaml` nếu thay đổi chỉ dành cho môi trường lab. Giữ
base trung lập giúp nhiều lớp phủ khác nhau cùng tái sử dụng một cấu hình gốc.

## Dọn dẹp

Xóa đúng các tài nguyên được tạo từ lớp phủ:

```bash
kubectl delete -k deploy/labs/2.4/kustomize/overlays/lab
```
