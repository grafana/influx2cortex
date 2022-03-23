FROM golangci/golangci-lint:latest-alpine

RUN apk update && apk add --upgrade jq curl
