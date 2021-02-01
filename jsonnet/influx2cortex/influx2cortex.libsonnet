local jaeger_mixin = import 'github.com/grafana/jsonnet-libs/jaeger-agent-mixin/jaeger.libsonnet';
local k = import 'github.com/grafana/jsonnet-libs/ksonnet-util/kausal.libsonnet',
      container = k.core.v1.container,
      containerPort = k.core.v1.containerPort,
      deployment = k.apps.v1.deployment;

{
  _images+:: {
    influx2cortex: 'ghcr.io/gouthamve/influx2cortex:sha-caecbb5',
  },

  _config+:: {
    influx2cortex: {
      replicas: 3,
      distributor_endpoint: 'dns:///distributor.svc:9095',
    },
  },

  influx2cortex_args:: {
    'distributor.endpoint': $._config.influx2cortex.distributor_endpoint,
  },

  influx2cortex_container::
    container.new('influx2cortex', $._images.influx2cortex) +
    container.withPorts([
      containerPort.newNamed(name='http-metrics', containerPort=80),
      containerPort.newNamed(name='grpc', containerPort=9095),
    ]) +
    container.withArgsMixin(k.util.mapToFlags($.influx2cortex_args)) +
    k.util.resourcesRequests('2', '1Gi') +
    k.util.resourcesLimits(null, '2Gi') +
    jaeger_mixin,

  influx2cortex_deployment:
    deployment.new('influx2cortex', $._config.influx2cortex.replicas, [$.influx2cortex_container]) +
    k.util.antiAffinity,

  influx2cortex_service:
    k.util.serviceFor($.influx2cortex_deployment),
}