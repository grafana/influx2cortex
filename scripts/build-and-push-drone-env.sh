#!/usr/bin/env bash
set -eufo pipefail

docker build --platform linux/amd64 -t i2c/drone-env:latest .drone/
docker tag i2c/drone-env:latest us.gcr.io/kubernetes-dev/influx2cortex/drone-env:latest
docker push us.gcr.io/kubernetes-dev/influx2cortex/drone-env:latest
