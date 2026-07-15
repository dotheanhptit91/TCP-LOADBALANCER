#!/bin/sh
set -eu

wait_for_worker() {
  service="$1"
  attempts=0
  while [ "$attempts" -lt 15 ]; do
    health="$(docker compose exec -T "$service" wget -qO- http://127.0.0.1:8080/healthz 2>/dev/null || true)"
    if echo "$health" | grep -Eq '"successful_connections":[1-9][0-9]*'; then
      echo "PASS $service established an end-to-end TCP session"
      return
    fi
    attempts=$((attempts + 1))
    sleep 1
  done
  echo "FAIL $service did not establish a successful session" >&2
  docker compose logs "$service" >&2
  exit 1
}

wait_for_worker tcp-backend-worker1
wait_for_worker tcp-backend-worker2

attempts=0
while [ "$attempts" -lt 15 ]; do
  rules="$(docker compose exec -T tcp-lb-gateway nft list table inet tcp_lb)"
  if echo "$rules" | grep -Eq 'tcp sport 1000-1010.*packets [1-9]' \
    && echo "$rules" | grep -Eq 'tcp dport 1000-1010.*packets [1-9]' \
    && echo "$rules" | grep -Eq 'tcp sport 2000-2010.*packets [1-9]' \
    && echo "$rules" | grep -Eq 'tcp dport 2000-2010.*packets [1-9]' \
    && echo "$rules" | grep -Eq 'tcp sport 1000-1010.*packets [1-9].*snat ip to 10\.30\.0\.254' \
    && echo "$rules" | grep -Eq 'tcp sport 2000-2010.*packets [1-9].*snat ip to 10\.30\.0\.254'; then
    break
  fi
  attempts=$((attempts + 1))
  sleep 1
done
if [ "$attempts" -eq 15 ]; then
  echo "FAIL nftables counters did not increase for all four routing rules" >&2
  echo "$rules" >&2
  exit 1
fi

server_logs="$(docker compose logs --no-color tcp-server-emu)"
echo "$server_logs" | grep -Eq 'remote_address=10\.30\.0\.254:10(0[0-9]|10)' || {
  echo "FAIL server did not observe gateway SNAT with a worker1 source port" >&2
  exit 1
}
echo "$server_logs" | grep -Eq 'remote_address=10\.30\.0\.254:20(0[0-9]|10)' || {
  echo "FAIL server did not observe gateway SNAT with a worker2 source port" >&2
  exit 1
}

echo "PASS nftables observed outbound and SYN-ACK/return packets for both ranges"
echo "PASS server observed gateway server-side IP and preserved worker source ports"
