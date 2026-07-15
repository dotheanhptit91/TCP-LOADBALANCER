# TCP routed load-balancing mini-project

Đây là data plane TCP L3/L4 trong đó `tcp-backend-worker` là **client** và `tcp-server-emu` là **server**. Gateway không terminate TCP và không tạo connection thứ hai.

## Packet flow

```text
worker1 10.10.0.x:1000-1010                 server 10.30.0.10:9000
worker2 10.20.0.x:2000-2010                 ┌─────────────────────┐
┌──────────────────────────┐                │   tcp-server-emu    │
│ tcp-backend-worker client│                └──────────▲──────────┘
└────────────┬─────────────┘                           │
             │ SYN: original worker IP/port            │ SYN: src=10.30.0.254
             ▼                                        │
        ┌──────────────────────────────────────────────┘
        │ tcp-lb-gateway: routing + nftables SNAT + conntrack
        └──────────────────────────────────────────────┐
             ▲                                        │
             │ reverse NAT về worker                   │ SYN-ACK tới 10.30.0.254
             └────────────────────────────────────────┘
```

Gateway áp dụng ba rule cho mỗi worker group:

- Worker → server: kiểm tra source CIDR và `tcp sport` thuộc range được cấp.
- Server → worker: kiểm tra destination CIDR, `tcp dport` thuộc range và conntrack state là `ESTABLISHED`.
- Postrouting: SNAT source IP của worker thành `GATEWAY_SERVER_IP`, đồng thời giữ source port trong range nếu tuple chưa bị trùng.

Server chỉ nhìn thấy IP phía server của gateway (`10.30.0.254`). Linux conntrack giữ mapping để SYN-ACK quay lại đúng worker. XDP/eBPF chưa cần ở quy mô demo này.

## Thành phần

- `tcp-backend-worker`: định kỳ chọn source port ngẫu nhiên, gửi TCP SYN và kiểm tra echo end-to-end.
- `tcp-lb-gateway`: Go control plane cài nftables rules, đọc conntrack và ghi mapping vào Redis.
- `tcp-server-emu`: TCP echo server, log gateway IP và source port đã được SNAT.
- `pod-state-manager`: khám phá replica qua DNS, đọc `/healthz` và lưu state vào Redis.
- Redis: `lb:port_mappings`, health, active/successful connection và instance state.

`internal/proxy` đã được bỏ vì gateway không còn là user-space TCP proxy.

## Chạy bằng Docker Compose

Gateway và endpoint cần capability `NET_ADMIN`; worker cần thêm `NET_BIND_SERVICE` vì range 1000 nằm dưới 1024. Compose đã cấu hình các capability này.

```bash
docker compose up --build -d
./scripts/smoke-test.sh
```

Smoke test kiểm tra:

1. Cả hai worker tạo được TCP session và nhận đúng echo.
2. nftables counter tăng cho cả outbound và return path.
3. Server nhìn thấy `10.30.0.254` và source port đúng range, không nhìn thấy worker IP.

Xem data plane và connection state:

```bash
docker compose exec tcp-lb-gateway nft list table inet tcp_lb
docker compose exec tcp-lb-gateway conntrack -L -p tcp
curl http://127.0.0.1:8080/healthz
make redis-state
```

Capture SYN/SYN-ACK trực tiếp tại gateway:

```bash
docker compose exec tcp-lb-gateway \
  timeout 10 tcpdump -n -i any 'tcp port 9000 and (tcp[tcpflags] & (tcp-syn|tcp-ack) != 0)'
```

Worker tự tạo connection mỗi 3 giây nên capture sẽ có traffic mà không cần client ngoài.

Scale worker:

```bash
docker compose up -d \
  --scale tcp-backend-worker1=3 \
  --scale tcp-backend-worker2=2
```

Lưu ý: sau SNAT, các replica trong cùng group dùng chung một gateway IP. Để luôn giữ nguyên source port, tổng số connection đồng thời tới cùng server tuple của một group không được vượt số port trong range (11). Scale lớn cần cấp thêm SNAT IP hoặc chia nhỏ port range theo replica.

## Network topology của Compose

| Network | CIDR | Gateway data-plane IP |
|---|---:|---:|
| worker1-net | `10.10.0.0/24` | `10.10.0.254` |
| worker2-net | `10.20.0.0/24` | `10.20.0.254` |
| server-net | `10.30.0.0/24` | `10.30.0.254` |
| management | `10.40.0.0/24` | `10.40.0.254` |

Worker có static route tới `10.30.0.0/24` qua gateway. Server trả lời trực tiếp tới `10.30.0.254`; gateway dùng conntrack reverse NAT để chuyển về worker. Vì worker và server không cùng Docker network, không có đường bypass gateway.

## Kubernetes

Transparent multi-homed routing cần CNI cung cấp nhiều interface. Manifest `deploy/kubernetes.yaml` dùng Multus bridge networks và phù hợp cho single-node demo.

Manifest cũng có `kind-secondary-network-setup` DaemonSet để loại ba secondary CIDR khỏi `KIND-MASQ-AGENT`; nếu thiếu rule này, Kind sẽ SNAT source IP của worker trước khi packet tới gateway.

Các Pod data-plane có watchdog cho interface Multus. Sau khi host/Kind reboot, nếu network namespace cũ bị mất interface secondary, watchdog dùng ServiceAccount `data-plane-self-healer` để xóa chính Pod đó. Deployment sẽ tạo Pod mới và Multus thực hiện lại CNI ADD; không cần rollout restart thủ công.

Chuẩn bị:

```bash
# Cài Multus CNI trước, sau đó chọn một node chạy toàn bộ data plane:
kubectl label node <node-name> tcp-lb-role=dataplane
kubectl apply -f deploy/kubernetes.yaml
```

Với production multi-node, thay Multus bridge bằng underlay/overlay CNI hỗ trợ route giữa node (ví dụ macvlan/ipvlan hoặc CNI eBPF phù hợp). Không dùng manifest proxy cũ vì nó sẽ terminate TCP tại gateway.

## Cấu hình chính

| Service | Biến | Ví dụ |
|---|---|---|
| gateway | `PORT_MAPPINGS` | `1000-1010=worker1@10.10.0.0/24` |
| gateway | `SERVER_CIDR` | `10.30.0.0/24` |
| gateway | `GATEWAY_SERVER_IP` | `10.30.0.254` |
| worker | `SOURCE_PORT_RANGE` | `1000-1010` |
| worker | `REMOTE_ADDR` | `10.30.0.10:9000` |
| worker/server | `STATIC_ROUTES` | `10.30.0.0/24=10.10.0.254` |
| worker | `CONNECT_INTERVAL` | `3s` |
| worker | `SESSION_DURATION` | `5s` |

Worker group `n` dùng range `n000-n010`, ví dụ worker3 dùng `3000-3010`. Để thêm group, cấp một worker subnet không overlap, thêm interface gateway, route worker và một entry mới trong `PORT_MAPPINGS`.
