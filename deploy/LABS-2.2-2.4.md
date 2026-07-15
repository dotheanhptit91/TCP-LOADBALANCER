# Labs 2.2–2.4 with pod-state-manager

## Lab 2.2 — Blue/Green Switch

```bash
sh scripts/lab-2.2.sh
```

Both `pod-state-manager-blue` and `pod-state-manager-green` run concurrently.
The single `pod-state-manager-active` Service initially selects `slot: blue`;
the script changes only that selector to `slot: green` and verifies the response.

## Lab 2.3 — Scale & HPA

```bash
sh scripts/lab-2.3.sh
```

The script scales `pod-state-manager-hpa` to 10 replicas before creating an
`autoscaling/v2` HPA with a 50% CPU utilization target. CPU requests are set,
which is required for percentage-based CPU utilization. On Kind, the script
installs Metrics Server v0.8.1 and uses `--kubelet-insecure-tls` for the local lab.

## Lab 2.4 — Kustomize Overlay

```bash
sh scripts/lab-2.4.sh
```

The base defines one replica using image tag `v1`. The lab overlay references
the base and changes the tag to `v2` and replica count to 3 without copying the
Deployment manifest.

```text
deploy/kustomize/pod-state-manager/
├── base/
│   ├── deployment.yaml
│   └── kustomization.yaml
└── overlays/
    └── lab/
        └── kustomization.yaml
```

## Cleanup

```bash
kubectl -n tcp-lb-mini delete deployment pod-state-manager-blue pod-state-manager-green pod-state-manager-hpa pod-state-manager-kustomize
kubectl -n tcp-lb-mini delete service pod-state-manager-active
kubectl -n tcp-lb-mini delete hpa pod-state-manager-hpa
```
