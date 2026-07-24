# Lab 3.4 — Namespace Quotas với pod-state-manager

Lab này áp dụng `LimitRange` và `ResourceQuota` vào namespace thật
`tcp-lb-mini`, sau đó tạo lại Pod của Deployment `pod-state-manager` để quan sát
default resources. Cuối cùng, lab thử tạo một Pod dùng cùng image nhưng request
CPU vượt quota và xác minh Kubernetes API từ chối Pod đó.

ResourceQuota không xóa hay giảm tài nguyên của Pod đang chạy. Nó kiểm soát các
yêu cầu tạo/cập nhật mới qua admission.

## Chạy toàn bộ lab

```bash
sh deploy/labs/3.4/run.sh
```

## LimitRange

[quota.yaml](quota.yaml) đặt default cho container không khai báo resources:

| Trường | CPU | Memory |
|---|---:|---:|
| Default request | `25m` | `32Mi` |
| Default limit | `100m` | `128Mi` |
| Minimum | `5m` | `8Mi` |
| Maximum | `10` | `1Gi` |

Deployment `pod-state-manager` không khai báo resources cho container `manager`.
Khi script restart rollout, admission controller bổ sung default request/limit
vào Pod mới. Deployment template gốc không bị sửa bởi LimitRange.

## ResourceQuota

Quota giới hạn tổng tài nguyên trong namespace:

```yaml
hard:
  requests.cpu: "4"
  requests.memory: 4Gi
  pods: "100"
```

Mức này đủ cho workload hiện tại nhưng Pod thử nghiệm request `5 CPU`, nên luôn
vượt riêng giới hạn `requests.cpu: 4` trước khi được tạo hoặc schedule.

## Pod bị từ chối

[rejected-pod.yaml](rejected-pod.yaml) dùng image thật
`tcp-lb-mini/pod-state-manager:v2` và yêu cầu:

```yaml
resources:
  requests:
    cpu: "5"
    memory: 32Mi
```

Thử thủ công:

```bash
kubectl create -f deploy/labs/3.4/rejected-pod.yaml
```

Kết quả mong đợi chứa:

```text
exceeded quota: tcp-lb-namespace-quota
requests.cpu
```

Pod `pod-state-manager-quota-rejected` không được lưu vào API và không xuất hiện
trong `kubectl get pods`.

## Xác minh theo cách CKAD

```bash
kubectl -n tcp-lb-mini get resourcequota
kubectl -n tcp-lb-mini describe resourcequota tcp-lb-namespace-quota
kubectl -n tcp-lb-mini get limitrange
kubectl -n tcp-lb-mini describe limitrange tcp-lb-container-limits

POD=$(kubectl -n tcp-lb-mini get pod -l app=pod-state-manager \
  --sort-by=.metadata.creationTimestamp \
  -o custom-columns=NAME:.metadata.name --no-headers | tail -n 1)

kubectl -n tcp-lb-mini get pod "$POD" \
  -o jsonpath='{.spec.containers[?(@.name=="manager")].resources}{"\n"}'
```

## Dọn dẹp

```bash
kubectl -n tcp-lb-mini delete pod pod-state-manager-quota-rejected \
  --ignore-not-found
kubectl delete -f deploy/labs/3.4/quota.yaml
```

Xóa LimitRange không gỡ resources đã được admission thêm vào Pod hiện tại. Các
default sẽ không còn được áp dụng cho Pod được tạo sau đó.
