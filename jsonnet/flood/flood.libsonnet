local jaeger_mixin = import 'github.com/grafana/jsonnet-libs/jaeger-agent-mixin/jaeger.libsonnet';
local k = import 'github.com/grafana/jsonnet-libs/ksonnet-util/kausal.libsonnet',
      container = k.core.v1.container,
      containerPort = k.core.v1.containerPort,
      deployment = k.apps.v1.deployment;

{
  _images+:: {
    flood: 'ghcr.io/gouthamve/flood:sha-e055b7f',
  },

  _config+:: {
    flood: {
      replicas: 3,
      distributor_endpoint: 'dns:///distributor.svc:9095',
    },
  },

  flood_args:: {
    'distributor.endpoint': $._config.flood.distributor_endpoint,
  },

  flood_container::
    container.new('flood', $._images.flood) +
    container.withPorts([
      containerPort.newNamed(name='http-metrics', containerPort=80),
    ]) +
    container.withArgsMixin(k.util.mapToFlags($.flood_args)) +
    k.util.resourcesRequests('2', '1Gi') +
    k.util.resourcesLimits(null, '2Gi') +
    jaeger_mixin,

  flood_deployment:
    deployment.new('influx-flood', $._config.flood.replicas, [$.flood_container]) +
    k.util.antiAffinity,

  flood_service:
    k.util.serviceFor($.flood_deployment),
}