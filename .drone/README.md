# [Drone CI](https://drone.grafana.net/grafana/influx2cortex)

## Drone CI configuration

The `drone.yml` file is a generated file; do not edit it directly.

To make changes, edit the `drone.jsonnet` file in this directory and then run the following commands:

```
# This generates a new drone.yml file
drone jsonnet --stream --format --source .drone/drone.jsonnet --target .drone/drone.yml

# This lints the newly-generated drone.yml file
drone lint --trusted .drone/drone.yml

# This signs the new drone.yml file
drone sign --save grafana/influx2cortex .drone/drone.yml
```

Alternatively, run the following `make` command from the project root directory:

```
make drone
```

## Drone CI environment

The test coverage step requires the `jq` tool to run. The `jq` tool is provided by the `Dockerfile` in this directory. If you need to update the image, make your changes to the `Dockerfile` and run the following to push the new image to GCR:

```
docker build --platform linux/amd64 -t i2c/drone-env:latest .

docker tag i2c/drone-env:latest us.gcr.io/kubernetes-dev/influx2cortex/drone-env:latest

docker push us.gcr.io/kubernetes-dev/influx2cortex/drone-env:latest
```
