#!/usr/bin/env bash
set -eufo pipefail

if [ -z "$GRAFANABOT_PAT" ]
then
  echo "Missing env var: GRAFANABOT_PAT"
  exit 1
fi
if [ -z "$DRONE_PULL_REQUEST" ]
then
  echo "Missing env var: DRONE_PULL_REQUEST"
  exit 1
fi

command -v jq >/dev/null 2>&1 || { echo "jq is not installed"; exit 1; }

AUTH="grafanabot:${GRAFANABOT_PAT}"
ACCEPT_HEADER="Accept: application/vnd.github.v3+json"
ENDPOINT="https://api.github.com/repos/grafana/influx2cortex/issues/${DRONE_PULL_REQUEST}/comments"
COMMENT_CONTENT=$1

if [ -z "$COMMENT_CONTENT" ]
then
  echo "Missing comment content; exiting early"
  exit 1
fi

BODY="{\"body\":$(echo "$COMMENT_CONTENT" | jq -aRs)}"
echo "$BODY" > tmp-pr-comment.out

curl -u "$AUTH" -X POST -H "$ACCEPT_HEADER" "$ENDPOINT" -d @tmp-pr-comment.out