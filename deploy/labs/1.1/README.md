# Lab 1.1 — The 60-Second Pod

Run the complete lab from the project root:

```bash
sh deploy/labs/1.1/run.sh
```

Run the command from the project root. It creates a standalone
`pod-state-manager` Pod with labels, environment variables, and resources:

```bash
kubectl -n tcp-lb-mini run tcp-lb-60-second-pod \
  --image=tcp-lb-mini/pod-state-manager:v2 \
  --image-pull-policy=Never \
  --restart=Never \
  --labels='app=tcp-lb-60-second-pod,component=pod-state-manager,lab=1.1,track=exam-speed' \
  --env='LAB_NAME=the-60-second-pod' \
  --env='REDIS_ADDR=redis:6379' \
  --env='POLL_INTERVAL=60s' \
  --env='MANAGED_SERVICES=tcp-lb-gateway=tcp-lb-gateway:8080,tcp-backend-worker1=tcp-backend-worker1:8080,tcp-backend-worker2=tcp-backend-worker2:8080' \
  --overrides='{"apiVersion":"v1","spec":{"containers":[{"name":"tcp-lb-60-second-pod","resources":{"requests":{"cpu":"25m","memory":"32Mi"},"limits":{"cpu":"100m","memory":"64Mi"}}}]}}' \
  --override-type=strategic
```

Add these flags to export YAML without creating the Pod:

```bash
--dry-run=client -o yaml
```

The exported result is stored in `deploy/labs/1.1/pod.yaml`.

## Exam-speed verification without an editor

```bash
kubectl -n tcp-lb-mini get pod tcp-lb-60-second-pod -o wide
kubectl -n tcp-lb-mini get pod tcp-lb-60-second-pod --show-labels
kubectl -n tcp-lb-mini get pod tcp-lb-60-second-pod \
  -o jsonpath='{.spec.containers[0].env}{"\n"}{.spec.containers[0].resources}{"\n"}'
kubectl -n tcp-lb-mini logs tcp-lb-60-second-pod --tail=10
```

Cleanup:

```bash
kubectl -n tcp-lb-mini delete pod tcp-lb-60-second-pod
```
