#!/bin/sh

# This script will result in 1 file being generated:
#   - lint.out

command -v golangci-lint >/dev/null 2>&1 || { echo "golangci-lint is not installed"; exit 1; }

echo 'Go lint report:' > lint.out
echo '<details>' >> lint.out
echo '<summary>Click to expand.</summary>' >> lint.out
echo '```' >> lint.out
golangci-lint run --issues-exit-code 0 >> lint.out
echo '```' >> lint.out
echo '</details>' >> lint.out
