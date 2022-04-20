local drone = import 'lib/drone/drone.libsonnet';
local images = import 'lib/drone/images.libsonnet';
local triggers = import 'lib/drone/triggers.libsonnet';
local vault = import 'lib/vault/vault.libsonnet';

local pipeline = drone.pipeline;
local step = drone.step;
local withInlineStep = drone.withInlineStep;
local withSteps = drone.withSteps;
local withStep = drone.withStep;

local dockerPluginName = 'plugins/gcr';

local dockerPluginBaseSettings = {
  registry: 'us.gcr.io',
  repo: 'kubernetes-dev/influx2cortex',
  json_key: {
    from_secret: 'gcr_admin',
  },
};

local apps = [
  'influx2cortex',
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
  step: step('build binaries', $.commands),
  commands: [
    'bash ./scripts/compile-commands.sh',
  ],
};

local generateTags = {
  step: step('generate tags', $.commands),
  commands: [
    'DOCKER_TAG=$(bash scripts/generate-tags.sh)',
    // `.tag` is the file consumed by the `deploy-image` plugin.
    'echo -n "$${DOCKER_TAG}" > .tag',
    // `.tags` is the file consumed by the Docker (GCR inluded) plugins to tag the built Docker image accordingly.
    'if test "${DRONE_SOURCE_BRANCH}" = "main"; then echo -n "$${DOCKER_TAG},latest" > .tags; else echo -n "$${DOCKER_TAG}" > .tags; fi',
    // Print the contents of .tags for debugging purposes.
    'tail -n +1 .tags',
  ],
};

local dockerBuilder = {
  // step builds the pipeline step to build and push a docker image
  step(app): step(
    '%s: build and push' % app,
    [],
    image=dockerBuilder.pluginName,
    settings=dockerBuilder.settings(app),
  ),

  pluginName: 'plugins/gcr',

  // settings generates the CI Pipeline step settings
  settings(app): {
    repo: $._repo(app),
    registry: $._registry,
    dockerfile: './cmd/Dockerfile',
    json_key: { from_secret: 'gcr_admin' },
    mirror: 'https://mirror.gcr.io',
    build_args: ['cmd=' + app],
  },

  // image generates the image for the given app
  image(app): $._registry + '/' + $._repo(app),

  _repo(app):: 'kubernetes-dev/' + app,
  _registry:: 'us.gcr.io',
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
      entrypoint: ['dockerd'],
      command: [
        '--tls=false',
        '--host=tcp://0.0.0.0:2375',
        '--registry-mirror=https://mirror.gcr.io',
      ],
      privileged: true,
    } + withDockerSockVolume,
  ],
  environment+: {
    DOCKERD_ROOTLESS_ROOTLESSKIT_FLAGS: '-p 0.0.0.0:2376:2376/tcp',
  },
  volumes+: [
    {
      name: 'dockersock',
      temp: {},
    },
  ],
};

local acceptance = {
  step: step('acceptance', $.commands, environment=$.environment) + withDockerSockVolume,
  commands: [
    'export ACCEPTANCE_DOCKER_TAG=$(cat .tag)',
    'echo $${ACCEPTANCE_DOCKER_TAG}',
    'make acceptance-tests',
  ],
  environment: {
    DOCKER_HOST: 'tcp://docker:2375',
    DOCKER_TLS_CERTDIR: '',
    ACCEPTANCE_CI: 'true',
    ACCEPTANCE_DOCKER_HOST: 'docker',
    ACCEPTANCE_DOCKER_AUTH_USERNAME: '_json_key',
    ACCEPTANCE_DOCKER_AUTH_PASSWORD: { from_secret: 'gcr_admin' },
  },
};

[
  pipeline('build')
  + withStep(generateTags.step)
  + withInlineStep('build + push', [], image=dockerPluginName, settings=dockerPluginBaseSettings)
  + withStep(buildBinaries.step)
  + withSteps([dockerBuilder.step(app) for app in apps])
  + imagePullSecrets
  + triggers.pr
  + triggers.main,

  pipeline('acceptance', depends_on=['build'])
  + withStep(generateTags.step)
  + withStep(acceptance.step)
  + imagePullSecrets
  + withDockerInDockerService
  + triggers.pr
  + triggers.main,

  pipeline('test', depends_on=['build'])
  + withStep(generateTags.step)
  + withInlineStep('test', ['bash ./scripts/test.sh'])
  + withDockerInDockerService
  + imagePullSecrets
  + triggers.pr
  + triggers.main,

  pipeline('launch influx argo workflow', depends_on=['build', 'acceptance'])
  + withInlineStep('check is latest commit', ['[ $(git rev-parse HEAD) = $(git rev-parse remotes/origin/main) ]'])
  + withStep(generateTags.step)
  + withStep(drone.step(
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