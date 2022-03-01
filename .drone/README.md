[Drone CI](https://drone.grafana.net/grafana/influx2cortex)

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
