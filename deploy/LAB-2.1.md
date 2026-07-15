# Lab 2.1 — Rolling Update & Rollback

The lab creates a separate three-replica Deployment in `tcp-lb-mini`, using the
project worker's `/healthz` endpoint to expose the version in `service`. It does not modify the
live gateway, server, or worker Deployments.

## Automated run

From the project root:

```bash
sh scripts/lab-2.1.sh
```

The script performs and monitors these stages:

1. Deploy `v1` and wait until all three replicas are available.
2. Roll to `v2` with `maxSurge: 1` and `maxUnavailable: 1`.
3. Deploy `bad`, whose container exits immediately.
4. Observe the expected rollout timeout while old `v2` Pods remain available.
5. Run `kubectl rollout undo` and wait for a healthy rollback.

## Useful CKAD commands

```bash
kubectl -n tcp-lb-mini rollout status deployment/tcp-lb-rollout-demo
kubectl -n tcp-lb-mini rollout history deployment/tcp-lb-rollout-demo
kubectl -n tcp-lb-mini get rs,pods -l app=tcp-lb-rollout-demo
kubectl -n tcp-lb-mini describe deployment tcp-lb-rollout-demo
kubectl -n tcp-lb-mini rollout undo deployment/tcp-lb-rollout-demo
```

Verify the version served after rollback:

```bash
kubectl -n tcp-lb-mini exec deployment/redis -- \
  wget -qO- http://tcp-lb-rollout-demo:8080/healthz
```

Expected JSON contains `"service":"rollout-demo-v2"`.

## Why traffic remains available

The rolling strategy permits one extra Pod (`maxSurge: 1`) and at most one
unavailable desired replica (`maxUnavailable: 1`). A bad new Pod never becomes
Ready, so Kubernetes does not remove every healthy `v2` Pod. Rollback restores
the previous ReplicaSet template.

## Cleanup

```bash
kubectl delete -f deploy/lab-2.1-rolling-update.yaml
```
