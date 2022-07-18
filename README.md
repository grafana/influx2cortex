<a href="https://drone.grafana.net/grafana/influx2cortex"><img src="https://drone.grafana.net/api/badges/grafana/influx2cortex/status.svg" alt="Drone CI" /></a>
<a href="https://goreportcard.com/report/github.com/grafana/influx2cortex"><img src="https://goreportcard.com/badge/github.com/grafana/influx2cortex" alt="Go Report Card" /></a>

# influx2cortex: An Influx Proxy for Cortex

influx2cortex is a proxy that accepts Influx Line protocol and writes it to Cortex.
While today it only accepts writes, I have plans to add Flux read support too!

## Influx Line Protocol ingestion and translation

The Influx write proxy accepts the ingest requests and then translates the incoming Influx metrics into Prometheus metrics. The name mapping scheme for the looks as follows:

    Influx metric: cpu_load_short,host=server01,region=us-west value=0.64 1658139550000000000

    Prometheus metric: cpu_load_short{__proxy_source__="influx",host="server01",region="us-west"}

## Building

To build the proxy:

($ indicates the command line prompt)

```
$ go mod tidy
$ make build
$ make test
```

This should place a build of `influx2cortex` in the `dist` subdirectory.

## Running

Here we show how to configure and run the Influx write proxy to talk to an existing Mimir installation running on port 9090 on localhost. If no existing Mimir installation is available, or you would like to quickly install a test installation then follow the [getting-started](https://grafana.com/docs/mimir/latest/operators-guide/getting-started/) instructions.

### Gathering required information

In order to configure a write proxy we need to know the following pieces of information at a minimum:
* The TCP port that the write proxy should listen on
* The endpoint for remote writes within Mimir

The default TCP port for the write proxy is 8000 however it is best to choose a unique non-default port, especially if you are going to be running multiple write proxies (Graphite, Datadog, Influx, etc) on the same host.

If Mimir is configured to listen on port 9009 on localhost then the remote write endpoint will be http://localhost:9009/api/v1/push

### An example invocation

(Pre-built binaries/docker images are on our list of things to do.)

To run the proxy:

```
$ dist/influx2cortex \
  -auth.enable=false \
  -server.http-listen-address 127.0.0.1 \
  -server.http-listen-port 8008 \
  -write-endpoint http://localhost:9009/api/v1/push
```

Details of configurable options are available in the `-help` output.

### Example metric send

```
$ NOW=`date +%s000000000` ; curl -H "Content-Type: application/json" "http://localhost:8007/api/v1/push/influx/write" -d 'cpu_load_short,host=server01,region=us-west value=0.64 $NOW'
```

The data can now be queried from Mimir via the HTTP API or via Grafana. To find the above series via the HTTP API we can issue:

```
$ curl -G http://localhost:9009/prometheus/api/v1/series -d 'match[]=cpu_load_short'
{"status":"success","data":[{"__name__":"cpu_load_short","__proxy_source__":"influx","host":"server01","region":"us-west"}]}
```

## Grafana Cloud as a destination

If the destination Mimir installation is part of a Grafana cloud instance the `-write-endpoint` argument should be of the form:
  -write-endpoint https://_username_:_password_@_grafana_net_instance_/api/v1/push
where the exact server details can be found on Prometheus instance details page for the stack on grafana.com

The _username_ is the numeric `Username / Instance ID`
The _password_ is the Grafana Cloud API Key with `metrics push` privileges/role.
The _grafana_net_instance_ is server part of the URL to push Prometheus metrics.

## Internal metrics

The influx2cortex binary exposes internal metrics on a `/metrics` endpoint on a separate port which can be scraped by a local prometheus installation. This is configurable with the `internalserver` command line options.

## Roadmap

1. Add tests for this.
2. Add the read path.
3. Support out of order ingests.
