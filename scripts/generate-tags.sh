#!/usr/bin/env bash
# vim: ai:ts=8:sw=8:noet
# Prints the docker tag as:
# 20210617094655-master-deadbeef # using commit date and changeset when no changes were made
# 20210617094655-master-modified # using current date and the string 'modified' instead of changeset.
# Does not print a newline at the end of output
set -eufo pipefail
export SHELLOPTS        # propagate set to children by default
IFS=$'\t\n'
umask 0077

# In GitHub Actions, GITHUB_HEAD_REF will be set to the PR branch if this is a PR build.
# Otherwise, if it's a push, the branch name will be in GITHUB_REF_NAME.
BRANCH="${DRONE_SOURCE_BRANCH:-${GITHUB_HEAD_REF:-${GITHUB_REF_NAME:-$(git branch --show-current)}}}"

if git diff-index --quiet HEAD --
then
  if [[ ${DRONE_COMMIT:-unset} != "unset" ]];
  then
    GIT_COMMIT="${DRONE_COMMIT}"
    >&2 echo "\$DRONE_COMMIT=${DRONE_COMMIT}"
  elif [[ ${GITHUB_SHA:-unset} != "unset" ]];
  then
    GIT_COMMIT="${GITHUB_SHA}"
    >&2 echo "\$GITHUB_SHA=${GITHUB_SHA}"
  else
    GIT_COMMIT="$(git rev-list -1 HEAD)"
    >&2 echo "\$DRONE_COMMIT and GITHUB_SHA are unset, using last git commit $GIT_COMMIT"
  fi
  # no changes
  UNIX_TIMESTAMP=$(git show -s --format=%ct "$GIT_COMMIT")
  GIT_COMMIT_SHORT="$(git rev-parse --short "$GIT_COMMIT")"
else
  # changes
  UNIX_TIMESTAMP=$(date +%s)
  GIT_COMMIT_SHORT="modified"
fi

# date -u --rfc-3339=seconds requires GNU date
# when running this on alpine, run `apk add coreutils` to get it
if [[ ${OSTYPE:-notdarwin} == "darwin"* ]]; then
  # For MacOS, we need to use `gdate` for GNU `date`; run `brew install coreutils` to get it
  ISO_TIMESTAMP=$(gdate -u --rfc-3339=seconds "-d@${UNIX_TIMESTAMP}" | sed "s/+.*$//g" | sed "s/[^0-9]*//g")
else
  ISO_TIMESTAMP=$(date -u --rfc-3339=seconds "-d@${UNIX_TIMESTAMP}" | sed "s/+.*$//g" | sed "s/[^0-9]*//g")
fi
DOCKER_TAG=$(echo "${ISO_TIMESTAMP}-${BRANCH}-${GIT_COMMIT_SHORT}" | tr "/" "_")
echo -n "${DOCKER_TAG}"
