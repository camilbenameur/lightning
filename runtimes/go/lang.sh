# shellcheck shell=sh disable=SC2034 # sourced by common/run, which uses SOURCE_FILE
SOURCE_FILE=main.go
# $GOCACHE must be writable, so it lives in /tmp, but its 256 shard directories
# link to the read-only cache of the pre-built standard library (see
# Dockerfile): go reads it as usual, and since cache writes are best effort it
# just does not keep the program's own objects. That takes milliseconds, where
# linking every entry takes a second. A fresh trim.txt saves go a scan of the
# whole cache for stale entries. XDG_CONFIG_HOME holds a go config with
# telemetry off. -s -w (no symbol table, no DWARF) speeds up linking; panics
# still report file:line.
build() {
  mkdir "$GOCACHE" &&
    ln -s /opt/lightning/gocache/* "$GOCACHE/" &&
    date +%s >"$GOCACHE/trim.txt" &&
    XDG_CONFIG_HOME=/opt/lightning/config go build -ldflags='-s -w' -o /tmp/main main.go
}
run() { exec /tmp/main; }
