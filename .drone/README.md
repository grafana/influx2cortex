[Drone](https://drone.grafana.net/grafana/influx2cortex)

To make changes, edit the `drone.jsonnet` file in this directory and then run the following commands:

```
# This generates a new drone.yml file
drone jsonnet --stream --format --source .drone/drone.jsonnet --target .drone/drone.yml

# This lints the newly-generated drone.yml file
drone lint --trusted .drone/drone.yml
```
