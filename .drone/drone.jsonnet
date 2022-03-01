local triggers = import 'lib/drone/triggers.libsonnet';
local pipeline = (import 'lib/drone/drone.libsonnet').pipeline;
local withInlineStep = (import 'lib/drone/drone.libsonnet').withInlineStep;
local vault = import 'lib/vault/vault.libsonnet';

local dockerPluginName = 'plugins/gcr';

local dockerPluginBaseSettings = {
  registry: 'us.gcr.io',
  repo: 'kubernetes-dev/influx2cortex',
  json_key: {
    from_secret: 'gcr_admin',
  },
};

local generateTags = [
  'DOCKER_TAG=$(bash scripts/generate-tags.sh)',
  // `.tags` is the file consumed by the Docker (GCR inluded) plugins to tag the built Docker image accordingly.
  // We escape any `/`s in the source branch name by replacing them with `_`.
  'if test "${DRONE_SOURCE_BRANCH}" = "master"; then echo -n "$${DOCKER_TAG},latest" > .tags; else echo -n "$${DOCKER_TAG}" > .tags; fi',
  // Print the contents of .tags for debugging purposes.
  'tail -n +1 .tags',
];

local withImagePullSecrets = {
  image_pull_secrets: ['dockerconfigjson'],
};

[
  pipeline('build')
  + withInlineStep('test', ['go test ./...'])
  + withInlineStep('generate tags', generateTags)
  + withInlineStep('build + push', [], image=dockerPluginName, settings=dockerPluginBaseSettings)
  + withImagePullSecrets
  + triggers.pr
  + triggers.main,
]
+ [
  vault.secret('dockerconfigjson', 'secret/data/common/gcr', '.dockerconfigjson'),
  vault.secret('gh_token', 'infra/data/ci/github/grafanabot', 'pat'),
  vault.secret('gcr_admin', 'infra/data/ci/gcr-admin', 'service-account'),

]
