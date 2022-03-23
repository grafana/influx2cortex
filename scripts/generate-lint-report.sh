#!/usr/bin/env bash
set -eufo pipefail

# This script will result in 1 file being generated:
#   - lint.out

echo 'Go lint report:' > lint.out
echo '```' >> lint.out
golangci-lint run >> lint.out
echo '```' >> lint.out
