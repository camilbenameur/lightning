#!/usr/bin/env bash
# Build the Lightning sandbox runtime images, one at a time.
#
# Usage: runtimes/build.sh [--push] [id...]
#
# Builds runtimes/<id>/Dockerfile (build context: runtimes/) for every given id,
# or for every language when none is given, and tags it ${PREFIX}<id>:${TAG}
# (default lightning-runtime-<id>:latest). --push pushes each image once built.
set -euo pipefail

PREFIX=${PREFIX:-lightning-runtime-}
TAG=${TAG:-latest}
root=$(cd "$(dirname "$0")" && pwd)

usage() {
  sed -n '2,8s/^# \{0,1\}//p' "$0"
  exit "$1"
}

push=false
ids=()
for arg in "$@"; do
  case $arg in
    --push) push=true ;;
    -h | --help) usage 0 ;;
    -*) printf 'unknown option: %s\n' "$arg" >&2 && usage 2 >&2 ;;
    *) ids+=("$arg") ;;
  esac
done
if [ ${#ids[@]} -eq 0 ]; then
  for dockerfile in "$root"/*/Dockerfile; do
    dir=${dockerfile%/Dockerfile}
    ids+=("${dir##*/}")
  done
fi

for id in "${ids[@]}"; do
  if [ ! -f "$root/$id/Dockerfile" ]; then
    printf 'unknown runtime: %s (no %s/%s/Dockerfile)\n' "$id" "$root" "$id" >&2
    exit 2
  fi
done

for id in "${ids[@]}"; do
  image=$PREFIX$id:$TAG
  printf '==> building %s\n' "$image"
  start=$SECONDS
  docker build --tag "$image" --file "$root/$id/Dockerfile" "$root"
  printf '==> built %s in %ss\n' "$image" "$((SECONDS - start))"
  if [ "$push" = true ]; then
    docker push "$image"
  fi
done
