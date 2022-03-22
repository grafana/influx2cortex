# [Drone CI](https://drone.grafana.net/grafana/influx2cortex)

## Drone CI configuration

The `drone.yml` file is a generated file; do not edit it directly.

To make changes, edit the `drone.jsonnet` file in this directory and then run the following `make` command

```
make drone
```

## Drone CI environment

The [`comment-pr.sh`](https://github.com/grafana/influx2cortex/blob/main/scripts/comment-pr.sh) script that Drone uses expects `jq` to be available. The default `golang:1.17` image does not provide `jq` so we build and push a new image that includes `jq` with the following `make` command:

```
make drone-env
```
