.PHONY: help

help:
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'

test: ## Run golang tests
	bash ./scripts/test.sh

coverage-output:
	go test ./... -coverprofile=cover.out

coverage-show-func:
	go tool cover -func cover.out

# .PHONY: build
# build: ## Build the grpc-cortex-gw docker image
# build:
# 	docker build --build-arg=revision=$(GIT_REVISION) -t jdbgrafana/grpc-cortex-gw .

# CI
drone:
	drone jsonnet --source .drone/drone.jsonnet --target .drone/drone.yml --stream --format
	drone lint .drone/drone.yml
	drone sign --save grafana/influx2cortex .drone/drone.yml
