#!/bin/bash
# Builds docker images for local usage (acceptance testing or local k8s)
set -eufo pipefail
export SHELLOPTS	# propagate set to children by default
IFS=$'\t\n'

echo "Test"
command -v docker >/dev/null 2>&1 || { echo 'Please install docker'; exit 1; }

echo "# Compiling go binaries"
bash ./scripts/compile-commands.sh
echo "done"

echo "# Building docker images"
# If this gets too slow, we should allow users to only build the images for one proxy
for cmd in influx2cortex
do
  docker build \
    -f ./cmd/Dockerfile \
    -t "us.gcr.io/kubernetes-dev/${cmd}:local" \
    --build-arg "cmd=${cmd}" \
    .
done