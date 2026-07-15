# Lab 1.3 — Jobs & CronJobs

This lab runs against the `tcp-lb-mini` namespace and uses the existing TCP
load-balancer health endpoints.

## Run the lab

```bash
kubectl apply -f deploy/lab-1.3-jobs.yaml
kubectl -n tcp-lb-mini wait --for=condition=complete \
  job/tcp-lb-one-off-check --timeout=120s
kubectl -n tcp-lb-mini logs job/tcp-lb-one-off-check

kubectl -n tcp-lb-mini get cronjob tcp-lb-health-check
kubectl -n tcp-lb-mini get jobs --watch
```

The CronJob runs every minute. Inspect the Jobs and their owner:

```bash
kubectl -n tcp-lb-mini get jobs -l app=tcp-lb-health-check
kubectl -n tcp-lb-mini get job -l app=tcp-lb-health-check \
  -o custom-columns=JOB:.metadata.name,OWNER:.metadata.ownerReferences[0].kind,COMPLETE:.status.succeeded
kubectl -n tcp-lb-mini get pods -l app=tcp-lb-health-check
```

Verify the timestamps written to Redis:

```bash
kubectl -n tcp-lb-mini exec deployment/redis -- \
  redis-cli MGET lab:job:last_success lab:cronjob:last_success
```

## Job, CronJob, and Deployment

| Resource | Desired state | Pod behavior | Typical use |
|---|---|---|---|
| Job | A fixed number of successful completions | Stops after success; retries failures up to `backoffLimit` | Migration, batch processing, one-off validation |
| CronJob | Create Jobs according to `schedule` | Each scheduled Job runs to completion | Backup, report, periodic health audit |
| Deployment | Keep a requested number of Pods continuously running | Replaces Pods indefinitely; has no completion state | API server, worker service, long-running daemon |

`restartPolicy: Never` controls whether kubelet restarts the container in the
same Pod. `backoffLimit` controls how many failed Pods the Job controller
allows before the Job itself is marked failed.

## Cleanup

```bash
kubectl delete -f deploy/lab-1.3-jobs.yaml
```
