.PHONY: help

help:
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'

test: ## Run golang tests
	go test -race ./...

coverage-output:
	go test ./... -coverprofile=cover.out

coverage-show-func:
	go tool cover -func cover.out

protobuf: ## Runs protoc command to generate pb files
	bash ./scripts/genprotobuf.sh

# .PHONY: build
# build: ## Build the grpc-cortex-gw docker image
# build:
# 	docker build --build-arg=revision=$(GIT_REVISION) -t jdbgrafana/grpc-cortex-gw .

# CI
drone:
	scripts/generate-drone-yml.sh
