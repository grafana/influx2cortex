#!/bin/bash
# Builds command binaries
set -eufo pipefail
export SHELLOPTS	# propagate set to children by default
IFS=$'\t\n'

command -v go >/dev/null 2>&1 || { echo 'Please install go'; exit 1; }

export GOPRIVATE=""
export CGO_ENABLED=0
export GOOS=linux
export GOARCH=amd64
GIT_COMMIT="${DRONE_COMMIT:-$(git rev-list -1 HEAD)}"
COMMIT_UNIX_TIMESTAMP="$(git --no-pager show -s --format=%ct "${GIT_COMMIT}")"
# DOCKER_TAG="$(bash scripts/docker-tag.sh)"
DOCKER_TAG="TODO"

# shellcheck disable=SC2043
for cmd in influx2cortex
do
    go build \
    -o "dist/${cmd}" \
    -ldflags "\
        -w \
        -X 'github.com/grafana/mimir-graphite/pkg/appcommon.CommitUnixTimestamp=${COMMIT_UNIX_TIMESTAMP}' \
        -X 'github.com/grafana/mimir-graphite/pkg/appcommon.DockerTag=${DOCKER_TAG}' \
        " \
    "github.com/grafana/influx2cortex/cmd/${cmd}"

    echo "Succesfully built ${cmd} into dist/${cmd}"
done
