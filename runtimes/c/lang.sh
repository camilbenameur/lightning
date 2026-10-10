# shellcheck shell=sh disable=SC2034 # sourced by common/run, which uses SOURCE_FILE
SOURCE_FILE=main.c
# gnu17 is C17 that keeps the POSIX names (M_PI, getline, strdup...) which a
# strict -std=c17 hides.
build() { gcc -std=gnu17 -O2 -Wall -pipe -o /tmp/main main.c /opt/lightning/linebuf.o -lm; }
run() { exec /tmp/main; }
