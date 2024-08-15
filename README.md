<a href="https://github.com/grafana/influx2cortex/actions/workflows/build-and-push.yml?query=branch%3Amain"><img src="https://github.com/grafana/influx2cortex/actions/workflows/build-and-push.yml/badge.svg" alt="Build And Push" /></a>

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
  -server.http-listen-port 8007 \
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
  `-write-endpoint https://_username_:_password_@_grafana_net_instance_/api/v1/push`
where the exact server details can be found on Prometheus instance details page for the stack on grafana.com

The `_username_` is the numeric `Username / Instance ID`
The `_password_` is a Grafana Cloud API Key with the `MetricsPublisher` role.
The `_grafana_net_instance_` is server part of the URL to push Prometheus metrics.

## Configuring telegraf

Telegraf can be configured in two ways to send data to the Influx proxy.
- Using the `http` plugin
- Using the `influxdb` plugin

If connecting to a local influx proxy running on `localhost:8000` the two configs would look something like this:

### outputs.influxdb

Note: The url has a path of `/api/v1/push/influx`. The trailing `/write` is not required as this is appended by the `telegraf` agent when using the `outputs.influxdb` output plugin.

```
[[outputs.influxdb]]
  urls = ["https://localhost:8000/api/v1/push/influx"]
  data_format = "influx"
  skip_database_creation = true
```

The `skip_database_creation = true` option is to prevent errors such as:

```
022-09-27T16:20:20Z W! [outputs.influxdb] When writing to [https://localhost:8000/api/v1/push/influx]: database "telegraf" creation failed: 500 Internal Server Error
```

### outputs.http

Note: The url has a path of `/api/v1/push/influx/write`. The trailing `/write` is required as the `outputs.http` output plugin uses the URL without modification (unlike the `outputs.influxdb` output plugin above).

```
[[outputs.http]]
  url = "http://localhost:8000/api/v1/push/influx/write"
  data_format = "influx"
  timeout = "10s"
  method = "POST"
  interval = "300s"
  flush_interval = "150s"
```

## Configuring telegraf for Grafana Cloud

If you are a Grafana Cloud customer and wish to use telegraf to write to an Influx Proxy running inside Grafana Cloud you can use a config similar to this:

```
[[outputs.influxdb]]
  urls = ["https://_grafana_net_instance_/api/v1/push/influx/write"]
  username = "_username_"
  password = "_password_"
  data_format = "influx"
  skip_database_creation = true
```

As above, the `_username_`, `_password_` and `_grafana_net_instance_` are adapted from the Prometheus instance details for the stack information page on [grafana.com](https://grafana.com/).

For example, if your username was `123456789` and the Prometheus write endpoint was listed as `https://prometheus-prod-26-prod-ap-south-0.grafana.net/api/prom/push` then the corresponding config to send to Grafana Cloud would look something like:

```
[[outputs.influxdb]]
  urls = ["https://influx-prod-26-prod-ap-south-0.grafana.net/api/v1/push/influx/write"]
  username = "123456789"
  password = "_ELIDED_"
  data_format = "influx"
  skip_database_creation = true
```

Note: The hostname in the URL has `influx` instead of `prometheus`.

## More information

More information, including example `python`, `ruby` and `Node.js` snippets to push to the influx2cortex proxy can be found in the [Push metrics from Influx Telegraf to Prometheus](https://grafana.com/docs/grafana-cloud/data-configuration/metrics/metrics-influxdb/push-from-telegraf/#pushing-from-applications-directly) blog post.

## Internal metrics

The influx2cortex binary exposes internal metrics on a `/metrics` endpoint on a separate port which can be scraped by a local prometheus installation. This is configurable with the `internalserver` command line options.

## TODO - package consolidation
* Consolidate `pkg/internalserver' into mimir-proxies
