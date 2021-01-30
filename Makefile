# .ONESHELL:
# .DELETE_ON_ERROR:
# SHELL       := bash
# SHELLOPTS   := -euf -o pipefail
# MAKEFLAGS   += --warn-undefined-variables
# MAKEFLAGS   += --no-builtin-rule

# # Adapted from https://suva.sh/posts/well-documented-makefiles/
# .PHONY: help
# help: ## Display this help
# help:
# 	@awk 'BEGIN {FS = ": ##"; printf "Usage:\n  make <target>\n\nTargets:\n"} /^[a-zA-Z0-9_\.\-\/%]+: ##/ { printf "  %-45s %s\n", $$1, $$2 }' $(MAKEFILE_LIST)

# GIT_REVISION := $(shell git rev-parse --short HEAD)

# .PHONY: build
# build: ## Build the grpc-cortex-gw docker image
# build:
# 	docker build --build-arg=revision=$(GIT_REVISION) -t jdbgrafana/grpc-cortex-gw .
