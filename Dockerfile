FROM golang:1.17.8-alpine as build
RUN apk add --update --no-cache git

WORKDIR /go/src/github.com/grafana/influx2cortex
COPY . .

RUN CGO_ENABLED=0 go build -o /bin/influx2cortex --ldflags "-w -extldflags '-static'"  github.com/grafana/influx2cortex/cmd/influx2cortex 


FROM alpine:3.12

RUN apk add --update --no-cache ca-certificates
RUN addgroup -g 1000 app && \
  adduser -u 1000 -h /app -G app -S app
WORKDIR /app
USER app

COPY --from=build /bin/influx2cortex /bin/influx2cortex
ENTRYPOINT [ "/bin/influx2cortex" ]