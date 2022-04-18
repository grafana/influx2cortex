#!/bin/bash
# Builds command binaries
set -eufo pipefail
export SHELLOPTS	# propagate set to children by default
IFS=$'\t\n'

command -v go >/dev/null 2>&1 || { echo 'Please install go'; exit 1; }

export CGO_ENABLED=0
export GOOS=linux
export GOARCH=amd64

for cmd in influx2cortex
do
    go build \
    -tags netgo \
    -o "dist/${cmd}" \
    -ldflags "\
        -w \
        -extldflags '-static' \
        " \
    "github.com/grafana/influx2cortex/cmd/${cmd}"

    echo "Succesfully built ${cmd} into dist/${cmd}"
done