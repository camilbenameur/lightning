#!/usr/bin/env bash
# Deploy or update Lightning on the server. Operates on the directory that holds compose.yaml
# and .env, i.e. the parent of this script (for example /opt/lightning).
#
#   scripts/deploy.sh               deploy the version pinned in .env (LIGHTNING_VERSION)
#   scripts/deploy.sh <tag>         pin <tag> in .env first (latest, sha-1a2b3c4, 1.2.0);
#                                   this is also how to roll back
#
# Pulls every image (sandbox runtimes included, so no run waits for a pull), recreates the
# services that changed, waits until they are healthy, then removes superseded Lightning images.
set -euo pipefail

all_languages="python javascript typescript go rust c cpp java ruby php bash"

log() { printf '[deploy %s] %s\n' "$(date -u +%H:%M:%S)" "$*"; }
die() { log "error: $*" >&2; exit 1; }

cd "$(dirname "${BASH_SOURCE[0]}")/.."
[[ -f compose.yaml ]] || die "no compose.yaml in $PWD"
[[ -f .env ]] || die "no .env in $PWD (start from .env.example)"
command -v docker >/dev/null || die "docker is not installed"

previous=$(sed -n 's/^LIGHTNING_VERSION=//p' .env | tail -n 1)
if (($# > 0)); then
  [[ $1 =~ ^[A-Za-z0-9_][A-Za-z0-9_.-]{0,127}$ ]] || die "invalid image tag: $1"
  if grep -q '^LIGHTNING_VERSION=' .env; then
    sed -i "s/^LIGHTNING_VERSION=.*/LIGHTNING_VERSION=$1/" .env
  else
    printf '\nLIGHTNING_VERSION=%s\n' "$1" >>.env
  fi
fi

# Resolved configuration (compose.yaml + .env + defaults).
config=$(docker compose config)
setting() { sed -nE "s/^ +$1: *\"?([^\"]*)\"?\$/\1/p" <<<"$config" | head -n 1; }
prefix=$(setting LIGHTNING_RUNTIME_IMAGE_PREFIX)
runtime_tag=$(setting LIGHTNING_RUNTIME_TAG)
languages=$(setting LIGHTNING_LANGUAGES)
languages=${languages//,/ }
languages=${languages:-$all_languages}
service_images=$(sed -nE 's/^ +image: *"?([^"]+)"?$/\1/p' <<<"$config")

log "deploying $(sed -nE 's#.*/lightning-api:##p' <<<"$service_images") (previous: ${previous:-unset})"
docker compose pull --quiet
runtime_images=""
for id in $languages; do
  log "pulling $prefix$id:$runtime_tag"
  docker pull --quiet "$prefix$id:$runtime_tag" >/dev/null
  runtime_images+="$prefix$id:$runtime_tag"$'\n'
done

log "starting services"
if ! docker compose up --detach --wait --wait-timeout 180 --remove-orphans; then
  docker compose ps
  docker compose logs --tail 40
  die "services did not become healthy; roll back with: $0 ${previous:-<previous tag>}"
fi
docker compose ps

# Remove other tags of the images this stack uses, then dangling Lightning layers.
keep=$(printf '%s\n%s' "$service_images" "$runtime_images" | sed '/^$/d')
repos=$(sed -E 's/:[^:/]+$//' <<<"$keep" | sort -u)
docker image ls --format '{{.Repository}}:{{.Tag}}' | while read -r ref; do
  [[ $ref == *:\<none\> ]] && continue
  grep -qxF "${ref%:*}" <<<"$repos" || continue
  grep -qxF "$ref" <<<"$keep" && continue
  if docker image rm "$ref" >/dev/null 2>&1; then log "removed $ref"; fi
done
docker image prune --force --filter "label=org.opencontainers.image.source=https://github.com/camilbenameur/lightning" >/dev/null
log "done"
