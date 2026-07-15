# Lab 2.1 — Rolling Update & Rollback: pod-state-manager

This version of Lab 2.1 operates on the real `pod-state-manager` Deployment in
the `tcp-lb-mini` namespace.

```bash
sh scripts/lab-2.1-pod-state-manager.sh
```

The script performs the following operations:

1. Configures `maxSurge: 1`, `maxUnavailable: 0`, and `minReadySeconds: 5` for zero downtime with one replica.
2. Rolls `pod-state-manager:v1` to `pod-state-manager:v2`.
3. Deploys `pod-state-manager:bad` with `POLL_INTERVAL=not-a-duration`.
4. Confirms the bad Pod crashes while the previous `v2` Pod stays available.
5. Runs `kubectl rollout undo` and verifies that state updates continue.

Monitor the rollout in another terminal:

```bash
kubectl -n tcp-lb-mini rollout status deployment/pod-state-manager
kubectl -n tcp-lb-mini get rs,pods -l app=pod-state-manager \
  -L lab.tcp-lb/version --watch
kubectl -n tcp-lb-mini rollout history deployment/pod-state-manager
```

The simulated failure is an application configuration error, not an image-pull
failure, so its log clearly shows:

```text
invalid poll interval
```

`minReadySeconds` matters here because `pod-state-manager` has no HTTP
readiness endpoint. Without it, a fast-crashing container can briefly become
Ready and let Kubernetes consider the bad rollout successful before it exits.
