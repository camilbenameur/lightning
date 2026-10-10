# shellcheck shell=sh disable=SC2034 # sourced by common/run, which uses SOURCE_FILE
SOURCE_FILE=main.rs
# opt-level 2 gives optimized code at a moderate compile time; leaving the
# standard library's debug info out of the link (strip=debuginfo) halves it.
# Panic messages still name the file and line.
build() {
  rustc --edition 2024 -C opt-level=2 -C strip=debuginfo \
    -C linker=ld -C linker-flavor=ld -C link-self-contained=yes \
    -o /tmp/main main.rs
}
run() { exec /tmp/main; }
