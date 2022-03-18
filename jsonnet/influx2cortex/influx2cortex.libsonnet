local externalSecrets = import 'external-secrets/main.libsonnet';
local jaeger_mixin = import 'github.com/grafana/jsonnet-libs/jaeger-agent-mixin/jaeger.libsonnet';
local k = import 'github.com/grafana/jsonnet-libs/ksonnet-util/kausal.libsonnet',
      container = k.core.v1.container,
      containerPort = k.core.v1.containerPort,
      deployment = k.apps.v1.deployment,
      service = k.core.v1.service;

{
  _images+:: {
    influx2cortex: error 'must specify image',
  },

  secrets+: {
    gcr: externalSecrets.mapGCRSecret(),
  },

  _config+:: {
    influx2cortex: {
      replicas: 3,
      write_endpoint: 'dns://cortex-gw-internal.svc/api/prom/push',
    },
  },

  influx2cortex_args:: {
    'write-endpoint': $._config.influx2cortex.write_endpoint,
    'server.http-listen-port': '8080',
  },

  influx2cortex_container::
    container.new('influx2cortex', $._images.influx2cortex) +
    container.withPorts([
      containerPort.newNamed(name='http-metrics', containerPort=8080),
      containerPort.newNamed(name='grpc', containerPort=9095),
    ]) +
    container.withArgsMixin(k.util.mapToFlags($.influx2cortex_args)) +
    k.util.resourcesRequests('2', '1Gi') +
    k.util.resourcesLimits(null, '2Gi') +
    jaeger_mixin,

  influx2cortex_deployment:
    deployment.new('influx2cortex', $._config.influx2cortex.replicas, [$.influx2cortex_container])
    + deployment.mixin.spec.template.spec.withImagePullSecrets({ name: $.secrets.gcr.metadata.name })
    + k.util.antiAffinity,

  influx2cortex_service:
    k.util.serviceFor($.influx2cortex_deployment) +
    service.mixin.spec.withClusterIp('None'),
}
