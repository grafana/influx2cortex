#!/bin/bash
# Builds docker images for local usage (acceptance testing or local k8s)
set -eufo pipefail
export SHELLOPTS	# propagate set to children by default
IFS=$'\t\n'

echo "Test"
command -v docker >/dev/null 2>&1 || { echo 'Please install docker'; exit 1; }

# Populate a dummy .tag file for consumption by the Dockerfile
echo "local" > .tag
trap 'rm .tag' EXIT

echo "# Building docker images"
docker build -f ./Dockerfile -t "us.gcr.io/kubernetes-dev/influx2cortex:local" .