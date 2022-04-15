local drone = import 'lib/drone/drone.libsonnet';
local images = import 'lib/drone/images.libsonnet';
local triggers = import 'lib/drone/triggers.libsonnet';
local vault = import 'lib/vault/vault.libsonnet';
local step = drone.step;

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
  // `.tag` is the file consumed by the `deploy-image` plugin.
  'echo -n "$${DOCKER_TAG}" > .tag',
  // `.tags` is the file consumed by the Docker (GCR inluded) plugins to tag the built Docker image accordingly.
  'if test "${DRONE_SOURCE_BRANCH}" = "main"; then echo -n "$${DOCKER_TAG},latest" > .tags; else echo -n "$${DOCKER_TAG}" > .tags; fi',
  // Print the contents of .tags for debugging purposes.
  'tail -n +1 .tags',
];

local commentCoverageLintReport = [
  // Build drone utilities.
  'scripts/build-drone-utilities.sh',
  // Generate the raw coverage report.
  'go test -coverprofile=coverage.out ./...',
  // Process the raw coverage report.
  '.drone/coverage > coverage_report.out',
  // Generate the lint report.
  'scripts/generate-lint-report.sh',
  // Combine the reports.
  'cat coverage_report.out > report.out',
  'echo "" >> report.out',
  'cat lint.out >> report.out',
  // Submit the comment to GitHub.
  '.drone/ghcomment -id "Go coverage report:" -bodyfile report.out',
];

local imagePullSecrets = { image_pull_secrets: ['dockerconfigjson'] };

local buildBinaries = {
  step: step('build binaries', $.commands, image=images.go),
  commands: [
    'bash ./scripts/compile_commands.sh',
  ],
};

local withDockerSockVolume = {
  volumes+: [
    {
      name: 'dockersock',
      path: '/var/run',
    },
  ],
};

local withDockerInDockerService = {
  services: [
    {
      name: 'docker',
      image: images._images.dind,
      entrypoint: ['dockerd-rootless.sh'],
      command: [
        '--tls=false',
        '--host=tcp://0.0.0.0:2376',
        '--registry-mirror=https://mirror.gcr.io',
      ],
      privileged: false,
    } + withDockerSockVolume,
  ],
  environment+: {
    DOCKERD_ROOTLESS_ROOTLESSKIT_FLAGS: "-p 0.0.0.0:2376:2376/tcp",
  },
  volumes+: [
    {
      name: 'dockersock',
      temp: {},
    },
  ],
};

[
  drone.pipeline('pr')
  + drone.withInlineStep('generate tags', generateTags)
  + drone.withInlineStep('test', [    
    'export ACCEPTANCE_DOCKER_TAG=$(cat .tag)',
    'echo $${ACCEPTANCE_DOCKER_TAG}',
    'sleep 60',
    'go test ./...'])
  + drone.withInlineStep('coverage + lint', commentCoverageLintReport, image=images._images.goLint, environment={
    environment: {
      GRAFANABOT_PAT: { from_secret: 'gh_token' },
      DOCKER_HOST: 'tcp://docker:2375',
      DOCKER_TLS_CERTDIR: '',
      ACCEPTANCE_CI: 'true',
      ACCEPTANCE_DOCKER_HOST: 'docker',
      ACCEPTANCE_DOCKER_AUTH_USERNAME: '_json_key',
      ACCEPTANCE_DOCKER_AUTH_PASSWORD: { from_secret: 'gcr_admin' },
    },
  })
  + drone.withInlineStep('build + push', [], image=dockerPluginName, settings=dockerPluginBaseSettings)
  + imagePullSecrets
  + withDockerInDockerService
  + triggers.pr,

  drone.pipeline('main')
  + drone.withInlineStep('generate tags', generateTags)
  + drone.withInlineStep('test', ['go test ./...'])
  + withDockerInDockerService
  + drone.withInlineStep('build + push', [], image=dockerPluginName, settings=dockerPluginBaseSettings)
  + imagePullSecrets
  + triggers.main,

  drone.pipeline('launch influx argo workflow', depends_on=['main'])
  + drone.withInlineStep('check is latest commit', ['[ $(git rev-parse HEAD) = $(git rev-parse remotes/origin/main) ]'])
  + drone.withInlineStep('generate tags', generateTags)
  + drone.withStep(drone.step(
    'launch argo workflow',
    commands=[],
    settings={
      namespace: 'influx-cd',
      token: { from_secret: 'argo_token' },
      command: std.strReplace(|||
        submit --from workflowtemplate/influx-deploy
        --name influx-deploy-$(cat .tag)
        --parameter dockertag=$(cat .tag)
        --parameter commit=${DRONE_COMMIT}
        --parameter commit_author=${DRONE_COMMIT_AUTHOR}
        --parameter commit_link=${DRONE_COMMIT_LINK}
      |||, '\n', ' '),
      add_ci_labels: true,
    },
    image=images._images.argoCli,
  ))
  + imagePullSecrets
  + triggers.excludeModifiedPaths([
    '.drone/**',
    '.gitignore',
    'README.md',
  ])
  + triggers.main,
]
+ [
  vault.secret('dockerconfigjson', 'secret/data/common/gcr', '.dockerconfigjson'),
  vault.secret('gh_token', 'infra/data/ci/github/grafanabot', 'pat'),
  vault.secret('gcr_admin', 'infra/data/ci/gcr-admin', 'service-account'),
  vault.secret('argo_token', 'infra/data/ci/argo-workflows/trigger-service-account', 'token'),
]