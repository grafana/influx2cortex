.PHONY: help

help:
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'

build: ## Build local binaries
	bash ./scripts/compile_commands.sh

build-local: ## Builds local versions of images
	bash ./scripts/build-local-images.sh

acceptance-tests: ## Runs the acceptance tests, expecting the images to be already built
	go test -count 1 -v -race -tags=acceptance ./acceptance/...

test: ## Run golang tests
	go test -race ./...

coverage-output:
	go test ./... -coverprofile=cover.out

coverage-show-func:
	go tool cover -func cover.out

packages-minor-autoupdate:
	go mod edit -json \
		| jq ".Require \
			| map(select(.Indirect | not).Path) \
			| map(select( \
				. != \"github.com/thanos-io/thanos\" \
			))" \
		| tr -d '\n' | tr -d '  '

.PHONY: assert-no-changed-files
assert-no-changed-files:
	@git update-index --refresh
	@git diff-index --quiet HEAD --