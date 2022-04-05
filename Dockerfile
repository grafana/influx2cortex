FROM golang:1.17.8-alpine as build
RUN apk add --update --no-cache git

WORKDIR /go/src/github.com/grafana/influx2cortex
COPY . .

ENV GIT_COMMIT="${DRONE_COMMIT:-$(git rev-list -1 HEAD)}"
ENV COMMIT_UNIX_TIMESTAMP="$(git show -s --format=%ct "${GIT_COMMIT}")"
ENV DOCKER_TAG="$(bash scripts/generate-tags.sh)"

RUN GOPRIVATE="github.com/grafana/*" CGO_ENABLED=0 \
    go build -o /bin/influx2cortex \
    --ldflags " \
      -w -extldflags \
      '-static' \
      -X 'github.com/grafana/influx2cortex/pkg/influx/recorder.CommitUnixTimestamp=${COMMIT_UNIX_TIMESTAMP}' \
      -X 'github.com/grafana/influx2cortex/pkg/influx/recorder.DockerTag=${DOCKER_TAG}'\
    " \
    github.com/grafana/influx2cortex/cmd/influx2cortex


FROM alpine:3.12

RUN apk add --update --no-cache ca-certificates
RUN addgroup -g 1000 app && \
  adduser -u 1000 -h /app -G app -S app
WORKDIR /app
USER app

COPY --from=build /bin/influx2cortex /bin/influx2cortex
ENTRYPOINT [ "/bin/influx2cortex" ]