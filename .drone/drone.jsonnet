local triggers = import 'lib/drone/triggers.libsonnet';
local pipeline = (import 'lib/drone/drone.libsonnet').pipeline;
local withInlineStep = (import 'lib/drone/drone.libsonnet').withInlineStep;
local images = import 'lib/drone/images.libsonnet';
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
  'DOCKER_TAG=$(bash scripts/generate-tags.sh)',  // `.tags` is the file consumed by the Docker (GCR inluded) plugins to tag the built Docker image accordingly.
  // We escape any `/`s in the source branch name by replacing them with `_`.
  'if test "${DRONE_SOURCE_BRANCH}" = "master"; then echo -n "$${DOCKER_TAG},latest" > .tags; else echo -n "$${DOCKER_TAG}" > .tags; fi',
  // Print the contents of .tags for debugging purposes.
  'tail -n +1 .tags',
];

local commentTestCoverage = [
  // Build drone utilities.
  'scripts/build-drone-utilities.sh',
  // Generate the raw coverage report.
  'go test -coverprofile=coverage.out ./...',
  // Process the raw coverage report.
  '.drone/coverage > coverage_report.out',
  // Submit the comment to Github.
  '.drone/ghcomment -i "Go coverage report:" -b coverage_report.out',
];

[
  pipeline('pr')
  + withInlineStep('test', ['go test ./...'])
  + withInlineStep('test coverage', commentTestCoverage, environment={
    environment: {
      GRAFANABOT_PAT: { from_secret: 'gh_token' },
    },
  })
  + withInlineStep('generate tags', generateTags)
  + withInlineStep('build + push', [], image=dockerPluginName, settings=dockerPluginBaseSettings)
  + { image_pull_secrets: ['dockerconfigjson'] }
  + triggers.pr,

  pipeline('main')
  + withInlineStep('test', ['go test ./...'])
  + withInlineStep('generate tags', generateTags)
  + withInlineStep('build + push', [], image=dockerPluginName, settings=dockerPluginBaseSettings)
  + { image_pull_secrets: ['dockerconfigjson'] }
  + triggers.main,
]
+ [
  vault.secret('dockerconfigjson', 'secret/data/common/gcr', '.dockerconfigjson'),
  vault.secret('gh_token', 'infra/data/ci/github/grafanabot', 'pat'),
  vault.secret('gcr_admin', 'infra/data/ci/gcr-admin', 'service-account'),
]
