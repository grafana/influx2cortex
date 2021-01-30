FROM golang:1.15-alpine3.12 as build
RUN apk add --update --no-cache git

WORKDIR /go/src/github.com/gouthamve/flood
COPY . .

RUN CGO_ENABLED=0 go build -o /bin/flood --ldflags "-w -extldflags '-static'"  github.com/gouthamve/flood/cmd/flood 


FROM alpine:3.12

RUN apk add --update --no-cache ca-certificates
RUN addgroup -g 1000 app && \
  adduser -u 1000 -h /app -G app -S app
WORKDIR /app
USER app

COPY --from=build /bin/flood /bin/flood
ENTRYPOINT [ "/bin/flood" ]