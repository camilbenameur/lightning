# shellcheck shell=sh disable=SC2034 # sourced by common/run, which uses SOURCE_FILE
SOURCE_FILE=main.cpp
# C++20 rather than 23: in C++23 mode <ostream> pulls in the whole std::format
# machinery, which doubles the compile time of every iostream program.
# The Dockerfile precompiles the most common first includes with cxx, so the
# flags match; GCC uses a header's .gch in /opt/lightning/pch when the program
# starts with that include, and falls back to the header otherwise.
cxx() { g++ -std=c++20 -O2 -Wall -pipe "$@"; }
build() { cxx -I/opt/lightning/pch -o /tmp/main main.cpp /opt/lightning/linebuf.o; }
run() { exec /tmp/main; }
