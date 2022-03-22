#!/usr/bin/env bash
set -eufo pipefail

# This script will result in 3 files being generated:
#   - coverage.out
#   - coverage_report.raw
#   - coverate_report.out

# Generate a coverage report.
go test -coverprofile=coverage.out ./...
go tool cover -func=coverage.out > coverage_report.raw

# To be readable in Github, we'll want to use markdown.
echo "Go coverage report:" > coverage_report.out
echo "| Source | Func | % |" >> coverage_report.out
echo "| ------ | ---- | - |" >> coverage_report.out

# Smash the content of `coverage_report.raw` into a markdown table:
#  - Replace tabs w/ `|`s
#  - Prepend `|` to each line
#  - Append `|` to each line
cat coverage_report.raw | sed -E 's/\t+/ | /g' | sed -E 's/^/| /g' | sed -E 's/$/ |/g' >> coverage_report.out
