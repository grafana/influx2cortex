# influx2cortex: An Influx Proxy for Cortex

influx2cortex is a proxy that accepts Influx Line protocol and writes it to Cortex.
While today it only accepts writes, I have plans to add Flux read support too!

To run it:

```
./influx2cortex -server.http-listen-port=8080 -auth.enable=false -distributor.endpoint=localhost:9095
```

## Roadmap

1. Add tests for this.
2. Add the read path.
3. Support out of order ingests.