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
  // Generate a coverage report.
  'go test -coverprofile=coverage.out ./...',
  'go tool cover -func=coverage.out > coverage_report.raw',
  // To be readable in Github, we'll want to use markdown.
  'echo "Go coverage report:" > coverage_report.out',
  'echo "| Source | Func | % |" >> coverage_report.out',
  'echo "| ------ | ---- | - |" >> coverage_report.out',
  // Smash the content of `coverage_report.raw` into a markdown table:
  //  - Replace tabs w/ `|`s
  //  - Prepend `|` to each line
  //  - Append `|` to each line
  "cat coverage_report.raw | sed -E 's/\t+/ | /g' | sed -E 's/^/| /g' | sed -E 's/$/ |/g' >> coverage_report.out",
  // Comment the output of the coverage report on the PR.
  'bash scripts/comment-pr.sh "`cat coverage_report.out`"',
];

[
  pipeline('build')
  + withInlineStep('test', ['go test ./...'])
  + withInlineStep('test coverage', commentTestCoverage, image=images._images.goWithJq, environment={
    environment: {
      GRAFANABOT_PAT: { from_secret: 'gh_token' },
    },
  })
  + withInlineStep('generate tags', generateTags)
  + withInlineStep('build + push', [], image=dockerPluginName, settings=dockerPluginBaseSettings)
  + { image_pull_secrets: ['dockerconfigjson'] }
  + triggers.pr
  + triggers.main,
]
+ [
  vault.secret('dockerconfigjson', 'secret/data/common/gcr', '.dockerconfigjson'),
  vault.secret('gh_token', 'infra/data/ci/github/grafanabot', 'pat'),
  vault.secret('gcr_admin', 'infra/data/ci/gcr-admin', 'service-account'),
]
