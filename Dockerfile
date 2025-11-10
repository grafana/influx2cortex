FROM golang:1.24.6-alpine@sha256:c8c5f95d64aa79b6547f3b626eb84b16a7ce18a139e3e9ca19a8c078b85ba80d AS build
RUN apk add --update --no-cache git coreutils

WORKDIR /go/src/github.com/grafana/influx2cortex
COPY . .

RUN GIT_COMMIT="${DRONE_COMMIT:-${GITHUB_SHA:-$(git rev-list -1 HEAD)}}"; \
    COMMIT_UNIX_TIMESTAMP="$(git --no-pager show -s --format=%ct "${GIT_COMMIT}")"; \
    DOCKER_TAG="$(cat .tag)"; \
    GOPRIVATE="github.com/grafana/*"; \
    CGO_ENABLED=0; \
    go build -o /bin/influx2cortex \
    --ldflags " \
      -w -extldflags \
      '-static' \
      -X 'github.com/grafana/influx2cortex/pkg/influx.CommitUnixTimestamp=${COMMIT_UNIX_TIMESTAMP}' \
      -X 'github.com/grafana/influx2cortex/pkg/influx.DockerTag=${DOCKER_TAG}'\
    " \
    github.com/grafana/influx2cortex/cmd/influx2cortex


RUN addgroup -g 1000 app && \
  adduser -u 1000 -h /app -G app -S app

FROM gcr.io/distroless/static-debian12@sha256:87bce11be0af225e4ca761c40babb06d6d559f5767fbf7dc3c47f0f1a466b92c

COPY --from=build /etc/passwd /etc/passwd
COPY --from=build /etc/group /etc/group

WORKDIR /app
USER app

COPY --from=build /bin/influx2cortex /bin/influx2cortex
ENTRYPOINT [ "/bin/influx2cortex" ]
