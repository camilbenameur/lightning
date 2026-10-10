# shellcheck shell=sh disable=SC2034 # sourced by common/run, which uses SOURCE_FILE
SOURCE_FILE=main.ts
# --no-install: never try to fetch missing packages (there is no network anyway).
run() { exec bun --no-install main.ts; }
