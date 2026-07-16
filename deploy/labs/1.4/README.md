# Lab 1.4 — Label & Annotation Drill

Run the complete drill:

```bash
sh deploy/labs/1.4/run.sh
```

The script creates five standalone Pods from one YAML `List` manifest, with
different `environment` and `track` labels, then updates and queries them in
bulk.

```bash
kubectl apply -f deploy/labs/1.4/pods.yaml
```

The YAML also defines the initial annotations:

```yaml
annotations:
  lab.tcp-lb/purpose: selector-practice
  lab.tcp-lb/owner: ckad
```

## CKAD speed commands

List every lab Pod and show selected labels:

```bash
kubectl -n tcp-lb-mini get pods -l lab=1.4 \
  -L environment,track,team,version
```

Equality selector:

```bash
kubectl -n tcp-lb-mini get pods \
  -l 'lab=1.4,environment=dev'
```

Set-based selector:

```bash
kubectl -n tcp-lb-mini get pods \
  -l 'lab=1.4,environment in (dev,staging)'
```

Inequality selector:

```bash
kubectl -n tcp-lb-mini get pods \
  -l 'lab=1.4,track!=stable'
```

Add or update labels across many objects:

```bash
kubectl -n tcp-lb-mini label pods -l lab=1.4 \
  team=platform version=v1 --overwrite

kubectl -n tcp-lb-mini label pods -l lab=1.4 \
  version=v2 --overwrite
```

Change only the canary Pods:

```bash
kubectl -n tcp-lb-mini label pods \
  -l 'lab=1.4,track=canary' track=stable --overwrite
```

Add and overwrite annotations in bulk:

```bash
kubectl -n tcp-lb-mini annotate pods -l lab=1.4 \
  lab.tcp-lb/purpose=selector-practice \
  lab.tcp-lb/owner=ckad --overwrite
```

Annotations cannot be selected with `-l`. Display them using custom columns:

```bash
kubectl -n tcp-lb-mini get pods -l lab=1.4 \
  -o custom-columns='NAME:.metadata.name,OWNER:.metadata.annotations.lab\.tcp-lb/owner'
```

Cleanup:

```bash
kubectl -n tcp-lb-mini delete pods -l lab=1.4
```
