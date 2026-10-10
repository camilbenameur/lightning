#!/usr/bin/env bash
# Smoke-test the Lightning runtime images under the exact sandbox flags the API uses.
#
# Usage: runtimes/test.sh [id...]
#
# Runs the fixture programs of runtimes/tests/<id>/ in ${PREFIX}<id>:${TAG}
# (default lightning-runtime-<id>:latest) for every given id, or for every
# language when none is given, prints a summary table with the median
# hello-world latency, and exits non-zero if any check failed.
#
# Fixtures (any extension, the run script names the source file):
#   hello*.*  prints exactly "Hello, world!" (every variant must pass)
#   echo.*    copies stdin to stdout line by line
#   exit.*    prints "bye" to stderr and exits with status 3
#   syntax.*  is invalid: non-zero exit, diagnostics on stderr, nothing on stdout
#   fs.*      tries to create each file of $FS_REPORT, printing "<path>: ok|error"
#   stream.*  prints "tick", sleeps for one second, then prints "tock"
set -uo pipefail

PREFIX=${PREFIX:-lightning-runtime-}
TAG=${TAG:-latest}
root=$(cd "$(dirname "$0")" && pwd)
work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT

HELLO=$'Hello, world!\n'
STDIN=$'héllo ⚡\nline2\n'
FS_REPORT=$'/etc/lightning: error\n/workspace/../lightning: error\n/workspace/lightning: ok\n/tmp/lightning: ok\n'
LATENCY_RUNS=5

usage() {
  sed -n '2,17s/^# \{0,1\}//p' "$0"
  exit "$1"
}

table_row() {
  printf '%-11s %-6s %-6s %-6s %-7s %-6s %-7s %9s %8s\n' "$@"
}

# create ID FILE [STDIN]: creates a container that runs FILE like the API does
# and prints its ID. Creating and starting separately (as the API does) keeps
# warnings that `docker run` prints, such as "memory limited without swap",
# out of the program's stderr.
create() {
  local mem=256m
  case $1 in go | java | rust) mem=512m ;; esac
  docker create --rm --init \
    --network none --read-only \
    --tmpfs /workspace:rw,exec,nosuid,nodev,size=64m,uid=10001,gid=10001,mode=0700 \
    --tmpfs /tmp:rw,exec,nosuid,nodev,size=128m,uid=10001,gid=10001,mode=1777 \
    --user 10001:10001 --cap-drop ALL --security-opt no-new-privileges \
    --memory "$mem" --memory-swap "$mem" --cpus 1 --pids-limit 128 \
    --ulimit nofile=256:256 --ulimit fsize=16777216:16777216 \
    --log-driver json-file --log-opt max-size=1m --log-opt max-file=1 \
    --label lightning.sandbox=true --hostname sandbox --workdir /workspace \
    -e LIGHTNING_SOURCE_B64="$(base64 <"$2" | tr -d '\n')" \
    -e LIGHTNING_STDIN_B64="$(printf '%s' "${3-}" | base64 | tr -d '\n')" \
    "$PREFIX$1:$TAG" /opt/lightning/run
}

# sandbox ID FILE [STDIN]: runs FILE; sets $status and leaves the program's
# stdout and stderr in $work/out and $work/err.
sandbox() {
  local container
  if container=$(create "$@" 2>"$work/err"); then
    docker start --attach "$container" >"$work/out" 2>"$work/err"
    status=$?
  else
    : >"$work/out"
    status=125
  fi
}

# expect ID CHECK WANT_STATUS WANT_STDOUT WANT_STDERR: judges the last run.
# WANT_STATUS "nonzero" and WANT_STDERR "*" accept any non-zero / non-empty value.
expect() {
  local problems=()
  case $3 in
    nonzero) [ "$status" -ne 0 ] || problems+=("exit status $status, want non-zero") ;;
    *) [ "$status" -eq "$3" ] || problems+=("exit status $status, want $3") ;;
  esac
  printf '%s' "$4" | cmp -s - "$work/out" || problems+=("unexpected stdout")
  if [ "$5" = '*' ]; then
    [ -s "$work/err" ] || problems+=("empty stderr, want diagnostics")
  else
    printf '%s' "$5" | cmp -s - "$work/err" || problems+=("unexpected stderr")
  fi
  [ ${#problems[@]} -eq 0 ] && return 0

  {
    printf 'FAIL %s %s: %s\n' "$1" "$2" "$(IFS=';' && echo "${problems[*]}")"
    printf '  stdout: %s\n' "$(head -c 800 "$work/out" | sed -n l | head -n 10)"
    printf '  stderr: %s\n' "$(head -c 800 "$work/err" | sed -n l | head -n 10)"
  } >&2
  return 1
}

# fixture ID NAME: prints the path of runtimes/tests/ID/NAME.*, or fails.
fixture() {
  local file
  for file in "$root/tests/$1/$2".*; do
    [ -f "$file" ] && echo "$file" && return
  done
  printf 'FAIL %s %s: missing fixture %s/tests/%s/%s.*\n' "$1" "$2" "$root" "$1" "$2" >&2
  return 1
}

# check ID NAME STDIN WANT_STATUS WANT_STDOUT WANT_STDERR: prints pass or FAIL.
check() {
  local file
  file=$(fixture "$1" "$2") || { echo FAIL && return; }
  sandbox "$1" "$file" "$3"
  if expect "$1" "$2" "$4" "$5" "$6"; then echo pass; else echo FAIL; fi
}

# hello ID: runs every hello fixture once, then the main one $LATENCY_RUNS
# more times; prints "pass|FAIL <median latency in ms>".
hello() {
  local file result=pass times=() start i
  for file in "$root/tests/$1"/hello*.*; do
    [ -f "$file" ] || continue
    sandbox "$1" "$file"
    expect "$1" "${file##*/}" 0 "$HELLO" '' || result=FAIL
  done
  file=$(fixture "$1" hello) || { echo FAIL - && return; }
  for ((i = 0; i < LATENCY_RUNS; i++)); do
    start=${EPOCHREALTIME//[.,]/}
    sandbox "$1" "$file"
    times+=($(((${EPOCHREALTIME//[.,]/} - start) / 1000)))
    expect "$1" "${file##*/}" 0 "$HELLO" '' || result=FAIL
  done
  echo "$result $(printf '%s\n' "${times[@]}" | sort -n | sed -n "$(((LATENCY_RUNS + 1) / 2))p")"
}

# stream ID: runs the stream fixture, noting when each line arrives; prints
# pass if "tock" came about a second after "tick", i.e. the output was not
# held back in a buffer (the usual default when stdout is a pipe) until exit.
stream() {
  local file container gap
  file=$(fixture "$1" stream) || { echo FAIL && return; }
  if container=$(create "$1" "$file" 2>"$work/err"); then
    docker start --attach "$container" 2>"$work/err" |
      while IFS= read -r line; do printf '%s %s\n' "${EPOCHREALTIME//[.,]/}" "$line"; done >"$work/arrivals"
    status=${PIPESTATUS[0]}
  else
    : >"$work/arrivals"
    status=125
  fi
  cut -d ' ' -f 2- "$work/arrivals" >"$work/out"
  expect "$1" stream 0 $'tick\ntock\n' '' || { echo FAIL && return; }
  gap=$(awk 'NR == 1 { t = $1 } NR == 2 { print int(($1 - t) / 1000) }' "$work/arrivals")
  if [ "$gap" -lt 500 ]; then
    printf 'FAIL %s stream: "tock" arrived %s ms after "tick", want ~1000 ms (stdout is buffered)\n' "$1" "$gap" >&2
    echo FAIL
    return
  fi
  echo pass
}

ids=()
for arg in "$@"; do
  case $arg in
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

rows=()
failed=0
for id in "${ids[@]}"; do
  if ! size=$(docker image inspect --format '{{.Size}}' "$PREFIX$id:$TAG" 2>/dev/null); then
    printf 'FAIL %s: image %s not found (build it with runtimes/build.sh %s)\n' "$id" "$PREFIX$id:$TAG" "$id" >&2
    rows+=("$(table_row "$id" FAIL FAIL FAIL FAIL FAIL FAIL - -)")
    failed=1
    continue
  fi
  read -r hello_result latency < <(hello "$id")
  results=(
    "$hello_result"
    "$(check "$id" echo "$STDIN" 0 "$STDIN" '')"
    "$(check "$id" exit '' 3 '' $'bye\n')"
    "$(check "$id" syntax '' nonzero '' '*')"
    "$(check "$id" fs '' 0 "$FS_REPORT" '')"
    "$(stream "$id")"
  )
  case " ${results[*]} " in *" FAIL "*) failed=1 ;; esac
  rows+=("$(table_row "$id" "${results[@]}" "$latency ms" "$((size / 1000000)) MB")")
done

echo
table_row LANGUAGE HELLO STDIN EXIT SYNTAX FS STREAM LATENCY SIZE
printf '%s\n' "${rows[@]}"
if [ "$failed" -ne 0 ]; then
  echo "some checks failed" >&2
  exit 1
fi
echo "all checks passed"
