# Bài thực hành 2.2 — Triển khai Blue/Green

## Mục tiêu

Bài thực hành minh họa cách triển khai một phiên bản mới mà không thay đổi trực
tiếp phiên bản đang phục vụ. Hai môi trường Blue và Green chạy song song; một
Service duy nhất quyết định môi trường nào nhận lưu lượng bằng label selector.

Sau khi hoàn thành, bạn sẽ biết cách:

- Chạy đồng thời hai phiên bản của cùng một ứng dụng.
- Chuyển toàn bộ lưu lượng bằng cách sửa selector của Service.
- Xác minh backend đang hoạt động qua EndpointSlice và phản hồi HTTP.
- Chuyển nhanh về phiên bản cũ nếu phiên bản mới gặp lỗi.

## Kiến trúc bài thực hành

| Thành phần | Image | Label | Vai trò |
|---|---|---|---|
| `pod-state-manager-blue` | `tcp-lb-mini/pod-state-manager:v1` | `slot: blue` | Phiên bản đang phục vụ ban đầu |
| `pod-state-manager-green` | `tcp-lb-mini/pod-state-manager:v2` | `slot: green` | Phiên bản mới cần kiểm tra |
| `pod-state-manager-active` | Không áp dụng | Chọn `slot: blue` hoặc `slot: green` | Điểm truy cập ổn định trên cổng `8080` |

Mỗi Pod có một container `release-status` trả về màu triển khai, phiên bản và
tên Pod. Readiness probe TCP chỉ đưa Pod vào danh sách endpoint sau khi cổng
`8080` sẵn sàng.

Luồng chuyển đổi:

```text
Trước khi chuyển:  Service -> slot: blue  -> pod-state-manager-blue  (v1)
Sau khi chuyển:    Service -> slot: green -> pod-state-manager-green (v2)
```

## Điều kiện tiên quyết

- Cluster Kubernetes đang hoạt động và `kubectl` trỏ tới đúng cluster.
- Namespace `tcp-lb-mini` đã tồn tại.
- Deployment `redis` đang chạy trong namespace này; script dùng Pod Redis làm
  máy khách để gọi Service từ bên trong cluster.
- Các image `tcp-lb-mini/pod-state-manager:v1` và `:v2` đã có trên node. Manifest
  sử dụng `imagePullPolicy: Never`, vì vậy Kubernetes không kéo image từ registry.

Kiểm tra nhanh:

```bash
kubectl cluster-info
kubectl get namespace tcp-lb-mini
kubectl -n tcp-lb-mini get deployment redis
```

Với cluster Kind tên `kind`, có thể nạp các image cục bộ bằng:

```bash
kind load docker-image --name kind \
  tcp-lb-mini/pod-state-manager:v1 \
  tcp-lb-mini/pod-state-manager:v2
```

## Chạy tự động

Từ thư mục gốc của dự án:

```bash
sh deploy/labs/2.2/run.sh
```

Kịch bản sẽ:

1. Áp dụng `deploy/labs/2.2/blue-green.yaml`.
2. Chờ cả hai Deployment Blue và Green sẵn sàng.
3. Gọi Service và xác minh phản hồi ban đầu chứa `color=blue version=v1`.
4. Đổi selector của Service từ `slot: blue` sang `slot: green`.
5. Thử lại tối đa 15 lần, mỗi lần cách nhau 2 giây, cho tới khi phản hồi chứa
   `color=green version=v2`.
6. Hiển thị trạng thái Deployment, Service và EndpointSlice cuối cùng.

## Thực hiện thủ công

Triển khai hai môi trường và chờ chúng sẵn sàng:

```bash
kubectl apply -f deploy/labs/2.2/blue-green.yaml
kubectl -n tcp-lb-mini rollout status deployment/pod-state-manager-blue
kubectl -n tcp-lb-mini rollout status deployment/pod-state-manager-green
```

Xác minh Service đang chọn Blue:

```bash
kubectl -n tcp-lb-mini get service pod-state-manager-active \
  -o jsonpath='{.spec.selector}'; echo
kubectl -n tcp-lb-mini exec deployment/redis -- \
  wget -qO- http://pod-state-manager-active:8080
```

Kết quả phản hồi phải có dạng:

```text
color=blue version=v1 pod=pod-state-manager-blue-...
```

Chuyển lưu lượng sang Green bằng cách chỉ sửa selector của Service:

```bash
kubectl -n tcp-lb-mini patch service/pod-state-manager-active \
  --type=merge \
  -p '{"spec":{"selector":{"app":"pod-state-manager-bg","slot":"green"}}}'
```

Kiểm tra lại backend và phản hồi:

```bash
kubectl -n tcp-lb-mini get endpointslice \
  -l kubernetes.io/service-name=pod-state-manager-active
kubectl -n tcp-lb-mini exec deployment/redis -- \
  wget -qO- http://pod-state-manager-active:8080
```

Phản hồi mới phải chứa `color=green version=v2`.

## Quay lại Blue

Nếu Green không đạt yêu cầu, chuyển selector về Blue mà không cần tạo lại Pod:

```bash
kubectl -n tcp-lb-mini patch service/pod-state-manager-active \
  --type=merge \
  -p '{"spec":{"selector":{"app":"pod-state-manager-bg","slot":"blue"}}}'
```

Blue/Green cho phép quay lại nhanh vì phiên bản cũ vẫn chạy. Đổi lại, phương pháp
này cần đủ tài nguyên để duy trì đồng thời cả hai môi trường trong thời gian triển khai.

## Dọn dẹp

```bash
kubectl delete -f deploy/labs/2.2/blue-green.yaml
```
