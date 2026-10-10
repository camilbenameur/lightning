#!/usr/bin/env bash
# End-to-end smoke test of a running Lightning deployment, through its public entry point (Caddy).
#
#   scripts/smoke.sh [BASE_URL]        default: http://localhost:8088 (the local compose stack)
#
# Checks the security headers, that private endpoints are not exposed, the metadata endpoints,
# a hello-world run for every enabled language, stdin, NDJSON streaming and error envelopes.
#
# Requires bash and curl. jq is used for the JSON assertions when installed; otherwise a
# sed-based fallback reads the same fields. SMOKE_INSECURE=1 skips TLS verification (for
# DOMAIN=localhost, which uses Caddy's self-signed local CA). Exits 1 if any check fails.
set -euo pipefail

base_url=${1:-http://localhost:8088}
base_url=${base_url%/}

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

curl_opts=(--silent --show-error --max-time 60)
if [[ ${SMOKE_INSECURE:-0} == 1 ]]; then curl_opts+=(--insecure); fi
if command -v jq >/dev/null 2>&1; then have_jq=1 parser=jq; else have_jq=0 parser=sed; fi

checks=0
failures=0

pass() { checks=$((checks + 1)); printf 'ok    %s\n' "$1"; }
fail() { checks=$((checks + 1)); failures=$((failures + 1)); printf 'FAIL  %s\n' "$1"; }

expect_eq() { # name actual expected
  if [[ $2 == "$3" ]]; then pass "$1"; else fail "$1 (got '$2', want '$3')"; fi
}
expect_contains() { # name haystack needle
  if [[ $2 == *"$3"* ]]; then pass "$1"; else fail "$1 ('$3' not found in: ${2:0:300})"; fi
}
expect_not_contains() { # name haystack needle
  if [[ $2 != *"$3"* ]]; then pass "$1"; else fail "$1 ('$3' found)"; fi
}
expect_set() { # name value
  if [[ -n $2 ]]; then pass "$1"; else fail "$1 (missing)"; fi
}

# header NAME: value of a response header of the last request (case-insensitive).
header() {
  grep -i "^$1:" "$tmp/headers" | tail -n 1 | cut -d: -f2- | tr -d '\r' | sed 's/^ *//' || true
}

# field JSON KEY: first scalar value stored under KEY, at any depth. Strings are unquoted
# (the fallback leaves JSON escapes such as \n in place, which substring checks tolerate).
field() {
  if ((have_jq)); then
    jq -r --arg k "$2" 'first(.. | objects | select(has($k)) | .[$k]) | if type == "string" then . else tojson end' <<<"$1" 2>/dev/null || true
  else
    sed -nE 's/.*"'"$2"'":("([^"\\]|\\.)*"|[^,}]*).*/\1/p' <<<"$1" | head -n 1 | sed -E 's/^"(.*)"$/\1/'
  fi
}

# json_str TEXT: TEXT as a JSON string literal (enough for the ASCII programs below).
json_str() {
  local s=$1
  s=${s//\\/\\\\}
  s=${s//\"/\\\"}
  s=${s//$'\t'/\\t}
  s=${s//$'\n'/\\n}
  printf '"%s"' "$s"
}

run_body() { # language code [stdin]
  printf '{"language":%s,"code":%s,"stdin":%s}' "$(json_str "$1")" "$(json_str "$2")" "$(json_str "${3:-}")"
}

# request METHOD PATH [JSON]: sets $status and $body (headers in $tmp/headers). Retries while
# the API answers 429 (honouring Retry-After) or 503 (queue full, runtime image still pulling).
request() {
  local method=$1 path=$2 data=${3:-} attempt delay
  local args=("${curl_opts[@]}" -X "$method" -D "$tmp/headers" -o "$tmp/body" -w '%{http_code}')
  if [[ -n $data ]]; then args+=(-H 'Content-Type: application/json' --data-binary "$data"); fi
  for attempt in 1 2 3 4 5 6 7 8; do
    : >"$tmp/headers"
    : >"$tmp/body"
    status=$(curl "${args[@]}" "$base_url$path") || status=000
    body=$(<"$tmp/body")
    case $status in
      429) delay=$(header retry-after) ;;
      503) delay=3 ;;
      *) return 0 ;;
    esac
    [[ $delay =~ ^[0-9]+$ ]] || delay=2
    if ((delay > 10)); then delay=10; fi
    printf '      %s %s -> %s, retrying in %ss (%d/8)\n' "$method" "$path" "$status" "$delay" "$attempt"
    sleep "$delay"
  done
}

hello_world() { # language id -> program printing "Hello, world!"
  case $1 in
    python) printf 'print("Hello, world!")\n' ;;
    javascript) printf 'console.log("Hello, world!");\n' ;;
    typescript) printf 'const greeting: string = "Hello, world!";\nconsole.log(greeting);\n' ;;
    go) printf 'package main\n\nimport "fmt"\n\nfunc main() {\n\tfmt.Println("Hello, world!")\n}\n' ;;
    rust) printf 'fn main() {\n    println!("Hello, world!");\n}\n' ;;
    c) printf '#include <stdio.h>\n\nint main(void) {\n    puts("Hello, world!");\n    return 0;\n}\n' ;;
    cpp) printf '#include <iostream>\n\nint main() {\n    std::cout << "Hello, world!" << std::endl;\n}\n' ;;
    java) printf 'public class Main {\n    public static void main(String[] args) {\n        System.out.println("Hello, world!");\n    }\n}\n' ;;
    ruby) printf 'puts "Hello, world!"\n' ;;
    php) printf '<?php\necho "Hello, world!\\n";\n' ;;
    bash) printf 'echo "Hello, world!"\n' ;;
    *) return 1 ;;
  esac
}

printf 'Smoke testing %s (JSON via %s)\n' "$base_url" "$parser"

# --- Edge: security headers, private endpoints ----------------------------------------------

request GET /
expect_eq "GET / status" "$status" 200
expect_contains "GET / serves HTML" "$(header content-type)" "text/html"
for name in content-security-policy strict-transport-security x-content-type-options \
  referrer-policy permissions-policy cross-origin-opener-policy; do
  expect_set "GET / header $name" "$(header "$name")"
done
expect_contains "CSP forbids framing" "$(header content-security-policy)" "frame-ancestors 'none'"
expect_eq "GET / hides the Server header" "$(header server)" ""

for path in /healthz /readyz /metrics; do
  request GET "$path"
  expect_not_contains "$path is not proxied to the API" "$body" '"status":"ok"'
  expect_not_contains "$path does not leak metrics" "$body" "# TYPE"
done

# --- Metadata -------------------------------------------------------------------------------

request GET /api/v1/info
expect_eq "GET /api/v1/info status" "$status" 200
expect_set "GET /api/v1/info version" "$(field "$body" version)"
expect_set "API responses carry X-Request-ID" "$(header x-request-id)"

request GET /api/v1/languages
expect_eq "GET /api/v1/languages status" "$status" 200
if ((have_jq)); then
  languages=$(jq -r '.languages[].id' <<<"$body")
else
  languages=$(grep -o '"id":"[^"]*"' <<<"$body" | cut -d'"' -f4)
fi
expect_set "at least one language is enabled" "$languages"

# --- Runs -----------------------------------------------------------------------------------

for id in $languages; do
  if ! code=$(hello_world "$id"); then
    fail "run $id (no hello-world program in this script)"
    continue
  fi
  request POST /api/v1/run "$(run_body "$id" "$code")"
  expect_eq "run $id status" "$status" 200
  expect_eq "run $id exitCode" "$(field "$body" exitCode)" 0
  expect_contains "run $id stdout" "$(field "$body" stdout)" "Hello, world!"
done

if [[ $'\n'$languages$'\n' == *$'\npython\n'* ]]; then
  request POST /api/v1/run "$(run_body python 'import sys; print(sys.stdin.read().upper())' lightning)"
  expect_contains "run python reads stdin" "$(field "$body" stdout)" "LIGHTNING"
fi

stream_lang=$(head -n 1 <<<"$languages")
if [[ -n $stream_lang ]]; then
  request POST /api/v1/run/stream "$(run_body "$stream_lang" "$(hello_world "$stream_lang")")"
  expect_eq "stream $stream_lang status" "$status" 200
  expect_contains "stream content type" "$(header content-type)" "application/x-ndjson"
  expect_eq "stream starts with a started event" "$(field "$(head -n 1 <<<"$body")" type)" started
  expect_contains "stream has a stdout event" "$(grep '"stdout"' <<<"$body" || true)" "Hello, world!"
  last=$(grep -v '^[[:space:]]*$' <<<"$body" | tail -n 1)
  expect_eq "stream ends with a result event" "$(field "$last" type)" result
  expect_eq "stream result exitCode" "$(field "$last" exitCode)" 0
fi

# --- Errors ---------------------------------------------------------------------------------

request POST /api/v1/run '{"language":"cobol","code":"DISPLAY \"hi\"."}'
expect_eq "unsupported language status" "$status" 400
expect_eq "unsupported language error code" "$(field "$body" code)" unsupported_language

request GET /api/v1/does-not-exist
expect_eq "unknown API route status" "$status" 404
expect_eq "unknown API route error code" "$(field "$body" code)" not_found

printf '\n%d checks, %d failed\n' "$checks" "$failures"
((failures == 0))
