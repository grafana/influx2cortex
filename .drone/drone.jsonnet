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

local generatePullRequestTags = [
  // `.tags` is the file consumed by the Docker (GCR inluded) plugins to tag the built Docker image accordingly.
  // We escape any `/`s in the source branch name by replacing them with `_`.
  'echo -n "${DRONE_SOURCE_BRANCH}-${DRONE_COMMIT_SHA}" | tr "/" "_" > .tags',
];

local generateMainTags = [
  // `.tags` is the file consumed by the Docker (GCR inluded) plugins to tag the built Docker image accordingly.
  // It is a comma-separated list of tags.
  'echo -n "${DRONE_BRANCH}-${DRONE_COMMIT_SHA},latest" > .tags',
];

local withImagePullSecrets = {
  image_pull_secrets: ['dockerconfigjson'],
};

[
  pipeline('pr')
  + withInlineStep('test', ['go test ./...'])
  + withInlineStep('generate tags', generatePullRequestTags)
  + withInlineStep('build', [], image=dockerPluginName, settings=dockerPluginBaseSettings)
  + withImagePullSecrets
  + triggers.pr,

  pipeline('main')
  + withInlineStep('test', ['go test ./...'])
  + withInlineStep('generate tags', generateMainTags)
  + withInlineStep('build + push', [], image=dockerPluginName, settings=dockerPluginBaseSettings)
  + withImagePullSecrets
  + triggers.main,
]
+ [
  vault.secret('dockerconfigjson', 'secret/data/common/gcr', '.dockerconfigjson'),
  vault.secret('gh_token', 'infra/data/ci/github/grafanabot', 'pat'),
  vault.secret('gcr_admin', 'infra/data/ci/gcr-admin', 'service-account'),

]
