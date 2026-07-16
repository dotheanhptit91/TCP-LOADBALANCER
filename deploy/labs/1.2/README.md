# Lab 1.2 — Init + Sidecar Pattern

Run the complete lab from the project root:

```bash
sh deploy/labs/1.2/run.sh
```

Apply the multi-container Pod:

```bash
kubectl apply -f deploy/labs/1.2/pod.yaml
kubectl -n tcp-lb-mini wait --for=condition=Ready \
  pod/tcp-lb-init-sidecar --timeout=120s
```

The Pod has three roles:

- `init-config` runs first and writes `runtime.env`, `init-status`, and `app.log`.
- `manager` sources `runtime.env`, runs the real `pod-state-manager`, and writes its output to `app.log`.
- `log-sidecar` mounts the same `emptyDir` read-only and follows `app.log`.

Verify the pattern without opening an editor:

```bash
kubectl -n tcp-lb-mini get pod tcp-lb-init-sidecar \
  -o jsonpath='{.status.initContainerStatuses[*].state.terminated.reason}{"\n"}{.status.containerStatuses[*].ready}{"\n"}'

kubectl -n tcp-lb-mini logs tcp-lb-init-sidecar -c init-config
kubectl -n tcp-lb-mini logs tcp-lb-init-sidecar -c manager --tail=10

kubectl -n tcp-lb-mini exec tcp-lb-init-sidecar -c manager -- \
  cat /shared/runtime.env /shared/init-status
kubectl -n tcp-lb-mini exec tcp-lb-init-sidecar -c log-sidecar -- \
  cat /shared/runtime.env /shared/init-status
```

## Tail application logs from the sidecar

The manager writes application output to `/shared/app.log`. The
`log-sidecar` container follows that file, so application logs can be read
through the sidecar with `kubectl logs -c`:

```bash
# Display recent application logs exposed by the sidecar.
kubectl -n tcp-lb-mini logs tcp-lb-init-sidecar \
  -c log-sidecar --tail=20

# Follow application logs continuously. Press Ctrl-C to stop.
kubectl -n tcp-lb-mini logs tcp-lb-init-sidecar \
  -c log-sidecar -f
```

Expected output includes `app loaded LAB_PATTERN=init-sidecar` followed by the
`instance state updated` messages produced by `pod-state-manager`.

Cleanup:

```bash
kubectl -n tcp-lb-mini delete pod tcp-lb-init-sidecar
```
