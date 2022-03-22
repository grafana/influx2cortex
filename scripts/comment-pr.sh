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
if [ -z "$DRONE_REPO_OWNER" ]
then
  echo "Missing env var: DRONE_REPO_OWNER"
  exit 1
fi
if [ -z "$DRONE_REPO_NAME" ]
then
  echo "Missing env var: DRONE_REPO_NAME"
  exit 1
fi

command -v jq >/dev/null 2>&1 || { echo "jq is not installed"; exit 1; }

MINIMIZE_COMMENT=0

while getopts 'mc:' OPTION
do
  case $OPTION in
    m) MINIMIZE_COMMENT=1
      ;;
    c) COMMENT="$OPTARG"
      ;;
    ?) echo "Unrecognized argument: $OPTION"
      exit 1
      ;;
  esac
done

if [ -z "$COMMENT" ]
then
  echo "Missing comment content; exiting early"
  exit 1
fi

AUTH_HEADER="Authorization: Bearer ${GRAFANABOT_PAT}"
ENDPOINT="https://api.github.com/graphql"

github_graphql () {
	PAYLOAD="{\"query\": $(echo $1 | jq -aRs)}"
	JQ_PROCESSING=$2

echo `curl -H "$AUTH_HEADER" -X POST -s -d "$PAYLOAD" "$ENDPOINT" | jq -r "$JQ_PROCESSING"`
}

# Get the node ID of the pull request

PR_QUERY="query { repository(owner: \"$DRONE_REPO_OWNER\", name: \"$DRONE_REPO_NAME\") { pullRequest(number: $DRONE_PULL_REQUEST) { id } } }"
PR_NODE_ID=`github_graphql "$PR_QUERY" '.data.repository.pullRequest.id'`

# Add comment

COMMENT_MUTATION="mutation { addComment(input: {subjectId: \"$PR_NODE_ID\", body: \"$COMMENT\"}) { commentEdge { node { id } } } }"
COMMENT_NODE_ID=`github_graphql "$COMMENT_MUTATION" '.data.addComment.commentEdge.node.id'`

# Minimize comment (optionally)
if [ "$MINIMIZE_COMMENT" -eq 1 ]; then
	MINIMIZE_MUTATION="mutation { minimizeComment(input: {subjectId: \"$COMMENT_NODE_ID\", classifier: RESOLVED}) { minimizedComment { isMinimized } } } "

	github_graphql "$MINIMIZE_MUTATION" '.data.minimizeComment.minimizedComment.isMinimized' > /dev/null
fi

echo 'Done.'
