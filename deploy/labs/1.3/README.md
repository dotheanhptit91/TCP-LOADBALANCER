# Bài thực hành 1.3 — Job và CronJob

Bài thực hành này chạy trong namespace `tcp-lb-mini` và sử dụng các điểm cuối
kiểm tra tình trạng hoạt động hiện có của bộ cân bằng tải TCP.

Chạy toàn bộ bài thực hành từ thư mục gốc dự án:

```bash
sh deploy/labs/1.3/run.sh
```

## Chạy bài thực hành

```bash
kubectl apply -f deploy/labs/1.3/jobs.yaml
kubectl -n tcp-lb-mini wait --for=condition=complete \
  job/tcp-lb-one-off-check --timeout=120s
kubectl -n tcp-lb-mini logs job/tcp-lb-one-off-check

kubectl -n tcp-lb-mini get cronjob tcp-lb-health-check
kubectl -n tcp-lb-mini get jobs --watch
```

CronJob chạy mỗi phút. Kiểm tra các Job và tài nguyên sở hữu chúng:

```bash
kubectl -n tcp-lb-mini get jobs -l app=tcp-lb-health-check
kubectl -n tcp-lb-mini get job -l app=tcp-lb-health-check \
  -o custom-columns=JOB:.metadata.name,OWNER:.metadata.ownerReferences[0].kind,COMPLETE:.status.succeeded
kubectl -n tcp-lb-mini get pods -l app=tcp-lb-health-check
```

Xác minh các dấu thời gian được ghi vào Redis:

```bash
kubectl -n tcp-lb-mini exec deployment/redis -- \
  redis-cli MGET lab:job:last_success lab:cronjob:last_success
```

## Job, CronJob và Deployment

| Tài nguyên | Trạng thái mong muốn | Hành vi của Pod | Mục đích sử dụng phổ biến |
|---|---|---|---|
| Job | Một số lượng lần hoàn thành thành công cố định | Dừng sau khi thành công; thử lại khi thất bại tối đa theo `backoffLimit` | Di chuyển dữ liệu, xử lý hàng loạt, xác minh một lần |
| CronJob | Tạo các Job theo `schedule` | Mỗi Job được lên lịch sẽ chạy cho tới khi hoàn thành | Sao lưu, báo cáo, kiểm tra tình trạng định kỳ |
| Deployment | Duy trì liên tục số lượng Pod được yêu cầu | Thay thế Pod không giới hạn; không có trạng thái hoàn thành | Máy chủ API, dịch vụ worker, tiến trình nền chạy lâu dài |

`restartPolicy: Never` kiểm soát việc kubelet có khởi động lại container trong
cùng một Pod hay không. `backoffLimit` kiểm soát số Pod được phép thất bại trước
khi bộ điều khiển Job đánh dấu chính Job đó là thất bại.

## Dọn dẹp

```bash
kubectl delete -f deploy/labs/1.3/jobs.yaml
```
