#!/usr/bin/env bash
set -eufo pipefail

TAG="${DRONE_ENV_TAG:-latest}"

build_and_push () {
  DRONE_ENV=$1

  docker build --platform linux/amd64 -t i2c/$DRONE_ENV:$TAG -f .drone/$DRONE_ENV.Dockerfile .drone/
  docker tag i2c/$DRONE_ENV:$TAG us.gcr.io/kubernetes-dev/influx2cortex/$DRONE_ENV:$TAG
  docker push us.gcr.io/kubernetes-dev/influx2cortex/$DRONE_ENV:$TAG
}

build_and_push coverage
build_and_push lint
