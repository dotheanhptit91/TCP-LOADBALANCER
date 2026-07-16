# Bài thực hành 2.1 — Cập nhật cuốn chiếu và quay lui: pod-state-manager

Phiên bản Bài thực hành 2.1 này thao tác trên Deployment `pod-state-manager`
thực tế trong namespace `tcp-lb-mini`.

```bash
sh deploy/labs/2.1/run-pod-state-manager.sh
```

Kịch bản thực hiện các thao tác sau:

1. Cấu hình `maxSurge: 1`, `maxUnavailable: 0` và `minReadySeconds: 5` để không có thời gian gián đoạn với một bản sao.
2. Cập nhật cuốn chiếu từ `pod-state-manager:v1` lên `pod-state-manager:v2`.
3. Triển khai `pod-state-manager:bad` với `POLL_INTERVAL=not-a-duration`.
4. Xác nhận Pod lỗi bị dừng trong khi Pod `v2` trước đó vẫn sẵn sàng phục vụ.
5. Chạy `kubectl rollout undo` và xác minh quá trình cập nhật trạng thái vẫn tiếp tục.

Theo dõi đợt triển khai trong một terminal khác:

```bash
kubectl -n tcp-lb-mini rollout status deployment/pod-state-manager
kubectl -n tcp-lb-mini get rs,pods -l app=pod-state-manager \
  -L lab.tcp-lb/version --watch
kubectl -n tcp-lb-mini rollout history deployment/pod-state-manager
```

Lỗi mô phỏng là lỗi cấu hình ứng dụng, không phải lỗi kéo image, vì vậy nhật ký
hiển thị rõ:

```text
invalid poll interval
```

`minReadySeconds` rất quan trọng ở đây vì `pod-state-manager` không có điểm cuối
HTTP kiểm tra mức độ sẵn sàng. Nếu thiếu thuộc tính này, một container bị lỗi
nhanh có thể chuyển sang trạng thái Ready trong chốc lát, khiến Kubernetes coi
đợt triển khai lỗi là thành công trước khi container thoát.
