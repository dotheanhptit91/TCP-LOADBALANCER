# Dự án nhỏ cân bằng tải TCP bằng định tuyến

Đây là mặt phẳng dữ liệu TCP L3/L4, trong đó `tcp-backend-worker` là **máy khách** và `tcp-server-emu` là **máy chủ**. Gateway không kết thúc kết nối TCP và không tạo kết nối thứ hai.

## Luồng gói tin

```text
worker1 10.10.0.x:1000-1010                 máy chủ 10.30.0.10:9000
worker2 10.20.0.x:2000-2010                 ┌─────────────────────┐
┌──────────────────────────┐                │   tcp-server-emu    │
│ tcp-backend-worker       │                └──────────▲──────────┘
└────────────┬─────────────┘                           │
             │ SYN: IP/cổng gốc của worker             │ SYN: nguồn=10.30.0.254
             ▼                                        │
        ┌──────────────────────────────────────────────┘
        │ tcp-lb-gateway: định tuyến + nftables SNAT + conntrack
        └──────────────────────────────────────────────┐
             ▲                                        │
             │ NAT ngược về worker                     │ SYN-ACK tới 10.30.0.254
             └────────────────────────────────────────┘
```

Gateway áp dụng ba quy tắc cho mỗi nhóm worker:

- Worker → máy chủ: kiểm tra CIDR nguồn và `tcp sport` thuộc dải được cấp.
- Máy chủ → worker: kiểm tra CIDR đích, `tcp dport` thuộc dải và trạng thái conntrack là `ESTABLISHED`.
- Hậu định tuyến: SNAT địa chỉ IP nguồn của worker thành `GATEWAY_SERVER_IP`, đồng thời giữ cổng nguồn trong dải nếu bộ giá trị kết nối chưa bị trùng.

Máy chủ chỉ nhìn thấy địa chỉ IP phía máy chủ của gateway (`10.30.0.254`). Conntrack của Linux lưu ánh xạ để SYN-ACK quay lại đúng worker. XDP/eBPF chưa cần thiết ở quy mô minh họa này.

## Thành phần

- `tcp-backend-worker`: định kỳ chọn cổng nguồn ngẫu nhiên, gửi TCP SYN và kiểm tra phản hồi echo xuyên suốt.
- `tcp-lb-gateway`: mặt phẳng điều khiển viết bằng Go, cài đặt các quy tắc nftables, đọc conntrack và ghi ánh xạ vào Redis.
- `tcp-server-emu`: máy chủ TCP echo, ghi nhật ký địa chỉ IP gateway và cổng nguồn đã được SNAT.
- `pod-state-manager`: khám phá các bản sao qua DNS, đọc `/healthz` và lưu trạng thái vào Redis.
- Redis: lưu `lb:port_mappings`, tình trạng hoạt động, các kết nối đang hoạt động/thành công và trạng thái của từng phiên bản.

`internal/proxy` đã được loại bỏ vì gateway không còn là proxy TCP ở không gian người dùng.

## Chạy bằng Docker Compose

Gateway và điểm cuối cần capability `NET_ADMIN`; worker cần thêm `NET_BIND_SERVICE` vì dải cổng bắt đầu từ 1000, thấp hơn 1024. Compose đã cấu hình các capability này.

```bash
docker compose up --build -d
./scripts/smoke-test.sh
```

Bài kiểm tra nhanh xác minh:

1. Cả hai worker tạo được phiên TCP và nhận đúng phản hồi echo.
2. Bộ đếm nftables tăng trên cả đường đi và đường về.
3. Máy chủ nhìn thấy `10.30.0.254` và cổng nguồn nằm đúng dải, nhưng không nhìn thấy địa chỉ IP của worker.

Xem mặt phẳng dữ liệu và trạng thái kết nối:

```bash
docker compose exec tcp-lb-gateway nft list table inet tcp_lb
docker compose exec tcp-lb-gateway conntrack -L -p tcp
curl http://127.0.0.1:8080/healthz
make redis-state
```

Bắt gói SYN/SYN-ACK trực tiếp tại gateway:

```bash
docker compose exec tcp-lb-gateway \
  timeout 10 tcpdump -n -i any 'tcp port 9000 and (tcp[tcpflags] & (tcp-syn|tcp-ack) != 0)'
```

Worker tự tạo kết nối mỗi 3 giây nên bản ghi bắt gói sẽ có lưu lượng mà không cần máy khách bên ngoài.

Thay đổi số lượng worker:

```bash
docker compose up -d \
  --scale tcp-backend-worker1=3 \
  --scale tcp-backend-worker2=2
```

Lưu ý: sau SNAT, các bản sao trong cùng một nhóm dùng chung địa chỉ IP gateway. Để luôn giữ nguyên cổng nguồn, tổng số kết nối đồng thời tới cùng một bộ giá trị máy chủ của một nhóm không được vượt quá số cổng trong dải (11). Khi tăng quy mô lớn, cần cấp thêm địa chỉ IP SNAT hoặc chia nhỏ dải cổng theo từng bản sao.

## Cấu trúc liên kết mạng của Compose

| Mạng | CIDR | IP mặt phẳng dữ liệu của gateway |
|---|---:|---:|
| worker1-net | `10.10.0.0/24` | `10.10.0.254` |
| worker2-net | `10.20.0.0/24` | `10.20.0.254` |
| server-net | `10.30.0.0/24` | `10.30.0.254` |
| management | `10.40.0.0/24` | `10.40.0.254` |

Worker có tuyến tĩnh tới `10.30.0.0/24` qua gateway. Máy chủ trả lời trực tiếp tới `10.30.0.254`; gateway dùng NAT ngược của conntrack để chuyển về worker. Vì worker và máy chủ không nằm trong cùng một mạng Docker nên không có đường đi vòng qua gateway.

## Kubernetes

Định tuyến trong suốt với nhiều kết nối mạng yêu cầu CNI cung cấp nhiều giao diện mạng. Manifest `deploy/kubernetes.yaml` sử dụng các mạng bridge của Multus và phù hợp với môi trường minh họa một node.

Manifest cũng có DaemonSet `kind-secondary-network-setup` để loại trừ ba CIDR phụ khỏi `KIND-MASQ-AGENT`; nếu thiếu quy tắc này, Kind sẽ SNAT địa chỉ IP nguồn của worker trước khi gói tin tới gateway.

Các Pod thuộc mặt phẳng dữ liệu có tiến trình giám sát giao diện Multus. Sau khi máy chủ/Kind khởi động lại, nếu không gian tên mạng cũ bị mất giao diện phụ, tiến trình giám sát dùng ServiceAccount `data-plane-self-healer` để xóa chính Pod đó. Deployment sẽ tạo Pod mới và Multus thực hiện lại thao tác CNI ADD; không cần khởi động lại đợt triển khai theo cách thủ công.

Chuẩn bị:

```bash
# Cài đặt Multus CNI trước, sau đó chọn một node chạy toàn bộ mặt phẳng dữ liệu:
kubectl label node <node-name> tcp-lb-role=dataplane
kubectl apply -f deploy/kubernetes.yaml
```

Với môi trường sản xuất nhiều node, hãy thay bridge Multus bằng CNI underlay/overlay hỗ trợ định tuyến giữa các node (ví dụ macvlan/ipvlan hoặc CNI eBPF phù hợp). Không dùng manifest proxy cũ vì nó sẽ kết thúc kết nối TCP tại gateway.

## Cấu hình chính

| Dịch vụ | Biến | Ví dụ |
|---|---|---|
| gateway | `PORT_MAPPINGS` | `1000-1010=worker1@10.10.0.0/24` |
| gateway | `SERVER_CIDR` | `10.30.0.0/24` |
| gateway | `GATEWAY_SERVER_IP` | `10.30.0.254` |
| worker | `SOURCE_PORT_RANGE` | `1000-1010` |
| worker | `REMOTE_ADDR` | `10.30.0.10:9000` |
| worker/máy chủ | `STATIC_ROUTES` | `10.30.0.0/24=10.10.0.254` |
| worker | `CONNECT_INTERVAL` | `3s` |
| worker | `SESSION_DURATION` | `5s` |

Nhóm worker `n` dùng dải `n000-n010`; ví dụ worker3 dùng `3000-3010`. Để thêm nhóm, hãy cấp một mạng con worker không chồng lấn, thêm giao diện gateway, tuyến worker và một mục mới trong `PORT_MAPPINGS`.
