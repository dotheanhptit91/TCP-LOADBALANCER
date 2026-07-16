#!/usr/bin/env sh
set -eu

namespace="${NAMESPACE:-tcp-lb-mini}"
manifest="deploy/labs/1.3/jobs.yaml"

echo "==> Removing the previous one-off Job"
kubectl -n "$namespace" delete job tcp-lb-one-off-check --ignore-not-found --wait=true

echo "==> Applying the Job and CronJob"
kubectl apply -f "$manifest"
kubectl -n "$namespace" wait --for=condition=complete \
  job/tcp-lb-one-off-check --timeout=120s
kubectl -n "$namespace" logs job/tcp-lb-one-off-check
kubectl -n "$namespace" get cronjob tcp-lb-health-check
kubectl -n "$namespace" get jobs -l lab=jobs-cronjobs
