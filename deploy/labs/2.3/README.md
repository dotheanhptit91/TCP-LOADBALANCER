# Bài thực hành 2.3 — Thay đổi quy mô và Horizontal Pod Autoscaler

## Mục tiêu

Bài thực hành gồm hai phần: thay đổi số lượng Pod thủ công và cấu hình Horizontal
Pod Autoscaler (HPA) để Kubernetes tự điều chỉnh số replica dựa trên mức sử dụng CPU.

Sau khi hoàn thành, bạn sẽ biết cách:

- Thay đổi số replica của Deployment bằng `kubectl scale`.
- Cấu hình CPU request để HPA tính được phần trăm sử dụng.
- Tạo HPA bằng API `autoscaling/v2`.
- Theo dõi số replica mong muốn, số replica hiện tại và số liệu CPU.

## Cấu hình được sử dụng

Deployment `pod-state-manager-hpa` bắt đầu với 2 replica và khai báo tài nguyên:

```yaml
resources:
  requests:
    cpu: 20m
    memory: 16Mi
  limits:
    cpu: 100m
    memory: 64Mi
```

HPA có các thông số chính:

| Thông số | Giá trị | Ý nghĩa |
|---|---:|---|
| `minReplicas` | 2 | Không giảm xuống dưới 2 Pod |
| `maxReplicas` | 10 | Không tăng vượt quá 10 Pod |
| `averageUtilization` | 50 | Duy trì mức sử dụng CPU trung bình gần 50% CPU request |
| `stabilizationWindowSeconds` | 30 | Chờ 30 giây trước khi quyết định giảm quy mô |

Phần trăm CPU của HPA được tính tương đối so với `resources.requests.cpu`, không
phải so với CPU limit hay tổng CPU của node. Với request `20m`, mức sử dụng
`10m` tương ứng khoảng 50%.

## Điều kiện tiên quyết

- Cluster Kubernetes và namespace `tcp-lb-mini` đang hoạt động.
- Image `tcp-lb-mini/pod-state-manager:v2` đã có trên node vì Deployment dùng
  `imagePullPolicy: Never`.
- Metrics API hoạt động để HPA đọc được số liệu CPU.
- Cluster có đủ tài nguyên để chạy tối đa 10 replica.

Kiểm tra Metrics API:

```bash
kubectl top nodes
kubectl top pods -n tcp-lb-mini
```

Nếu `kubectl top` chưa hoạt động, script tự cài Metrics Server v0.8.1. Trên Kind,
script thêm `--kubelet-insecure-tls` để Metrics Server kết nối được tới kubelet
dùng chứng chỉ cục bộ. Tùy chọn này chỉ phù hợp với môi trường thực hành, không
nên dùng làm cấu hình mặc định trong môi trường sản xuất.

## Chạy tự động

Từ thư mục gốc của dự án:

```bash
sh deploy/labs/2.3/run.sh
```

Kịch bản sẽ:

1. Kiểm tra `kubectl top nodes`.
2. Cài và chờ Metrics Server nếu Metrics API chưa sẵn sàng.
3. Áp dụng Deployment từ `deploy/labs/2.3/deployment.yaml`.
4. Tăng thủ công Deployment lên 10 replica và xác minh cả 10 replica đều Ready.
5. Áp dụng HPA từ `deploy/labs/2.3/hpa.yaml` với mục tiêu CPU 50%.
6. Chờ HPA nhận được CPU metrics rồi hiển thị HPA và CPU của từng Pod.

## Phần 1 — Thay đổi quy mô thủ công

Tạo Deployment:

```bash
kubectl apply -f deploy/labs/2.3/deployment.yaml
kubectl -n tcp-lb-mini rollout status deployment/pod-state-manager-hpa
```

Tăng lên 10 replica:

```bash
kubectl -n tcp-lb-mini scale deployment/pod-state-manager-hpa --replicas=10
kubectl -n tcp-lb-mini rollout status deployment/pod-state-manager-hpa
kubectl -n tcp-lb-mini get deployment,pods -l app=pod-state-manager-hpa
```

Lệnh `scale` sửa trường `.spec.replicas` của Deployment. ReplicaSet controller
sau đó tạo thêm Pod cho tới khi trạng thái thực tế khớp với trạng thái mong muốn.

## Phần 2 — Bật tự động thay đổi quy mô

Tạo HPA:

```bash
kubectl apply -f deploy/labs/2.3/hpa.yaml
kubectl -n tcp-lb-mini get hpa pod-state-manager-hpa
```

Theo dõi liên tục:

```bash
kubectl -n tcp-lb-mini get hpa pod-state-manager-hpa --watch
```

Trong một terminal khác, xem số liệu chi tiết:

```bash
kubectl -n tcp-lb-mini top pods -l app=pod-state-manager-hpa
kubectl -n tcp-lb-mini describe hpa pod-state-manager-hpa
```

Các cột quan trọng của `kubectl get hpa`:

- `TARGETS`: mức sử dụng hiện tại so với mục tiêu `50%`.
- `MINPODS` và `MAXPODS`: giới hạn số replica.
- `REPLICAS`: số replica hiện tại do HPA quan sát được.

Ứng dụng trong bài thực hành thường sử dụng ít CPU. Vì vậy, sau khi HPA được tạo,
nó có thể giảm dần từ 10 về `minReplicas: 2`. Đây là hành vi bình thường. Việc
tăng replica chỉ xảy ra khi mức CPU trung bình vượt mục tiêu đủ lâu.

## Xử lý sự cố

Nếu cột `TARGETS` hiển thị `<unknown>/50%`, kiểm tra:

```bash
kubectl get apiservice v1beta1.metrics.k8s.io
kubectl -n kube-system get pods -l k8s-app=metrics-server
kubectl -n kube-system logs deployment/metrics-server
kubectl -n tcp-lb-mini describe hpa pod-state-manager-hpa
```

Các nguyên nhân thường gặp là Metrics Server chưa sẵn sàng, Pod chưa có CPU
request, hoặc Pod mới chưa có đủ mẫu metrics.

## Dọn dẹp

Xóa HPA trước để nó không tiếp tục điều chỉnh Deployment:

```bash
kubectl -n tcp-lb-mini delete hpa pod-state-manager-hpa
kubectl -n tcp-lb-mini delete deployment pod-state-manager-hpa
```

Metrics Server là thành phần dùng chung của cluster nên bài thực hành không tự
động xóa thành phần này.
