{
  _images+:: {
    go: 'golang:1.17',
    testCoverageEnv: 'us.gcr.io/kubernetes-dev/influx2cortex/coverage:latest',
    lintEnv: 'us.gcr.io/kubernetes-dev/influx2cortex/lint:latest',
  },
}
