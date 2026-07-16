# Bài thực hành 2.1 — Cập nhật cuốn chiếu và quay lui

Bài thực hành tạo một Deployment riêng gồm ba bản sao trong `tcp-lb-mini`, sử dụng
điểm cuối `/healthz` của worker trong dự án để hiển thị phiên bản qua trường `service`.
Bài thực hành không sửa đổi các Deployment gateway, máy chủ hoặc worker đang chạy.

## Chạy tự động

Từ thư mục gốc của dự án:

```bash
sh deploy/labs/2.1/run-demo.sh
```

Kịch bản thực hiện và theo dõi các giai đoạn sau:

1. Triển khai `v1` và chờ cho tới khi cả ba bản sao đều sẵn sàng.
2. Cập nhật cuốn chiếu lên `v2` với `maxSurge: 1` và `maxUnavailable: 1`.
3. Triển khai `bad`, có container thoát ngay lập tức.
4. Quan sát đợt triển khai hết thời gian chờ như dự kiến trong khi các Pod `v2` cũ vẫn sẵn sàng.
5. Chạy `kubectl rollout undo` và chờ quá trình quay lui hoàn tất bình thường.

## Các lệnh CKAD hữu ích

```bash
kubectl -n tcp-lb-mini rollout status deployment/tcp-lb-rollout-demo
kubectl -n tcp-lb-mini rollout history deployment/tcp-lb-rollout-demo
kubectl -n tcp-lb-mini get rs,pods -l app=tcp-lb-rollout-demo
kubectl -n tcp-lb-mini describe deployment tcp-lb-rollout-demo
kubectl -n tcp-lb-mini rollout undo deployment/tcp-lb-rollout-demo
```

Xác minh phiên bản được phục vụ sau khi quay lui:

```bash
kubectl -n tcp-lb-mini exec deployment/redis -- \
  wget -qO- http://tcp-lb-rollout-demo:8080/healthz
```

Kết quả JSON dự kiến chứa `"service":"rollout-demo-v2"`.

## Vì sao lưu lượng vẫn được phục vụ

Chiến lược cập nhật cuốn chiếu cho phép thêm một Pod (`maxSurge: 1`) và có tối đa
một bản sao mong muốn không sẵn sàng (`maxUnavailable: 1`). Pod mới bị lỗi không
bao giờ chuyển sang trạng thái Ready, vì vậy Kubernetes không xóa toàn bộ Pod `v2`
đang hoạt động bình thường. Quá trình quay lui khôi phục mẫu ReplicaSet trước đó.

## Dọn dẹp

```bash
kubectl delete -f deploy/labs/2.1/rolling-update.yaml
```
