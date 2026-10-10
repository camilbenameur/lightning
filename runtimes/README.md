# Sandbox runtimes

One Docker image per language. The API runs every program in a fresh container
of its language's image, with no network, a read-only root filesystem and tight
resource limits, then throws the container away. An image only brings the
toolchain and a tiny launcher: the isolation comes from the container flags.

```
common/run          the launcher (/opt/lightning/run), shared by every image
common/linebuf.c    linked into C and C++ programs: line-buffered stdout
<id>/Dockerfile     the image; the build context is runtimes/
<id>/lang.sh        SOURCE_FILE, build() (optional) and run() for the launcher
tests/<id>/         fixture programs for test.sh
build.sh, test.sh   build and smoke-test the images
```

## Contract

What the API relies on, for every image:

- **Command** `/opt/lightning/run`, a POSIX `sh` script. User `runner`, uid/gid
  10001, `HOME=/tmp`, working directory `/workspace`.
- **Input** through the environment, base64-encoded (standard, padded):
  `LIGHTNING_SOURCE_B64` (required, may be empty) and `LIGHTNING_STDIN_B64`
  (optional). The launcher writes the source to `/workspace/<source file>` and
  the input to `/tmp/.stdin`, unsets both variables, builds the program if the
  language needs it, then `exec`s it with stdin redirected from the file.
- **Output** is the program's own: its stdout and stderr, plus compiler
  diagnostics on stderr. The launcher prints nothing itself.
- **Exit status** is the program's (`128 + n` when killed by signal `n`); a
  failed build exits with 1.
- **Files**: only the `/workspace` and `/tmp` tmpfs mounts are writable. Build
  caches are baked into the image, so nothing needs the network.
- **Streaming**: output is written as it is produced, line by line, even though
  stdout is a pipe (see [Output streaming](#output-streaming)).

The sandbox flags, applied by the API and by `test.sh` (`<mem>` is 512m for
go, java and rust, 256m otherwise):

```sh
docker run --rm --init \
  --network none --read-only \
  --tmpfs /workspace:rw,exec,nosuid,nodev,size=64m,uid=10001,gid=10001,mode=0700 \
  --tmpfs /tmp:rw,exec,nosuid,nodev,size=128m,uid=10001,gid=10001,mode=1777 \
  --user 10001:10001 --cap-drop ALL --security-opt no-new-privileges \
  --memory <mem> --memory-swap <mem> --cpus 1 --pids-limit 128 \
  --ulimit nofile=256:256 --ulimit fsize=16777216:16777216 \
  --log-driver json-file --log-opt max-size=1m --log-opt max-file=1 \
  --label lightning.sandbox=true --hostname sandbox --workdir /workspace \
  -e LIGHTNING_SOURCE_B64=<b64> -e LIGHTNING_STDIN_B64=<b64> \
  <image> /opt/lightning/run
```

Linux caps a single environment variable at 128 KiB, so a source file can be
at most 96 KiB (the API accepts 64 KiB).

## Languages

| id | toolchain | base image | build, then run (in `/workspace`) | image | hello world | peak memory |
|----|-----------|------------|-----------------------------------|------:|------------:|------------:|
| python | Python 3.14.7 | `python:3.14-alpine3.24` | `python3 -u main.py` | 67 MB | 0.62 s | 5 MiB |
| javascript | Node.js 24.21 | `node:24-alpine3.24` → `alpine:3.24` | `node main.js` | 143 MB | 0.93 s | 11 MiB |
| typescript | Bun 1.4.2 | `oven/bun:1.4-alpine` → `alpine:3.24` | `bun --no-install main.ts` | 85 MB | 0.59 s | 4 MiB |
| go | Go 1.27.1 | `golang:1.27-alpine3.24` → `alpine:3.24` | `go build -ldflags='-s -w' -o /tmp/main main.go`, `/tmp/main` | 298 MB | 1.3 s | 32 MiB |
| rust | Rust 1.98.1 | `rust:1.98-alpine3.24` → `alpine:3.24` | `rustc --edition 2024 -C opt-level=2 -C strip=debuginfo … -o /tmp/main main.rs`, `/tmp/main` | 471 MB | 1.0 s | 40 MiB |
| c | GCC 15.2 | `alpine:3.24` | `gcc -std=gnu17 -O2 -Wall -o /tmp/main main.c … -lm`, `/tmp/main` | 100 MB | 0.58 s | 8 MiB |
| cpp | GCC 15.2 | `alpine:3.24` | `g++ -std=c++20 -O2 -Wall -o /tmp/main main.cpp …`, `/tmp/main` | 338 MB | 0.88 s | 20 MiB |
| java | Temurin 25.0.4 | `eclipse-temurin:25-jdk-alpine-3.24` → `alpine:3.24` | `java -XX:AOTCache=… Main.java` | 180 MB | 1.4 s | 43 MiB |
| ruby | Ruby 3.4.11 | `ruby:3.4-alpine3.24` | `ruby -r/opt/lightning/sync.rb main.rb` | 77 MB | 0.73 s | 7 MiB |
| php | PHP 8.4.26 | `php:8.4-cli-alpine3.24` | `php main.php` | 111 MB | 0.85 s | 6 MiB |
| bash | Bash 5.3.9 | `alpine:3.24` | `bash main.sh` | 14 MB | 0.57 s | 4 MiB |

The exact commands are in each `<id>/lang.sh`. **Hello world** is the time
from `docker create` to exit for `tests/<id>/hello.*`, build included: the
median of the five sandboxed runs of `test.sh`, then the median of three
`test.sh` runs. **Peak memory** is the sandbox cgroup's `memory.peak` for the
same program, page cache warm. Heavier programs need more: a C++ program
using `<format>` peaks at about 160 MiB, one including `<bits/stdc++.h>` at
about 105 MiB, a Go program importing `net/http` at about 80 MiB.

The numbers come from a WSL2 VM (Ryzen 5 5600H, Docker 27.2) shared with other
busy workloads, where even an empty container takes from 0.5 s to several
seconds to start and exit depending on the load: single runs vary a lot, and
a dedicated server should do better.

### What makes each image small and fast

- **Go.** The whole standard library is pre-built into a read-only build cache
  at image build time, so a run only compiles the program itself: a program
  importing `net/http` builds in 1–2.5 s, against 57 s and a full `/tmp` with
  an empty cache. `go` wants a writable `GOCACHE`, so it is a directory in
  `/tmp` whose 256 shard directories link to the read-only cache: go reads
  them as usual and, cache writes being best effort, simply does not keep the
  program's own objects. Linking every entry instead costs about a second per
  run, and pointing `GOCACHE` at the read-only cache makes go scan all of it
  for stale entries after each build once the image is a day old. `-s -w`
  (no symbol table, no DWARF) cuts the build time by a quarter; panics still
  report `file:line`. The toolchain's own sources, tests, cgo, vet and other
  unused tools are dropped; telemetry is off and `GOPROXY=off`.
- **Rust.** Only `rustc` and the standard library of the host target are kept
  (no cargo, rustdoc, bundled lld, sanitizer runtimes or debug info). `rustc`
  links with binutils' `ld` directly, using the C runtime and musl that ship
  with the standard library (`-C link-self-contained=yes`), so the image needs
  no C compiler. Not linking the standard library's debug info
  (`-C strip=debuginfo`) halves the build time; `opt-level=2` balances compile
  time and speed.
- **C and C++.** GCC from Alpine, minus LTO, the LTO dump tool, plugin headers
  and sanitizer runtimes, which the fixed command line never uses: half of the
  toolchain's size. C uses `gnu17`, i.e. C17 keeping the POSIX names (`M_PI`,
  `getline`, `strdup`…) that a strict `-std=c17` hides. C++ uses C++20 rather
  than 23: in C++23 mode GCC 15's `<ostream>` pulls in the whole `std::format`
  machinery (65 000 instead of 40 000 lines to parse), which doubles the build
  time of every iostream program. Most of what is left is parsing the
  standard headers, so the image ships them precompiled for the two most
  common first lines of a program, `#include <iostream>` and
  `#include <bits/stdc++.h>` (built with the exact flags of `lang.sh`; GCC
  falls back to the plain headers for any other first include). That makes a
  hello world build in 0.3 s instead of 1.2–3 s, and a `<bits/stdc++.h>` one in
  2–5 s instead of 5–12 s, at less than half the memory. Code that formats
  with `std::format` stays slow to optimize: about 12 s for a one-liner at
  `-O2` on this machine.
- **Java.** A `jlink`ed runtime (Java SE, the compiler, locale data and
  `jdk.unsupported`) instead of the full JDK. The source launcher compiles
  `Main.java` in memory, so classic `public class Main` programs and JDK 25
  compact `void main()` ones both work. An AOT cache (JEP 483/514/515) is
  built at image build time from a training run of `java/Training.java`: it
  holds the JDK and javac classes already loaded and linked, which brings a
  hello world from 2.6 s to 1.1 s inside the container. Only the C1 JIT
  compiler runs: on one CPU, javac gets up to speed about 30 % sooner than
  with C1 and C2 (0.8 s instead of 1.1 s), at the cost of peak speed in long
  CPU-bound loops. Serial GC, no hsperfdata file, and JVM warnings go to
  stderr rather than stdout. The JVM sizes its heap from the container limit
  (a quarter of it by default).
- **Python.** The standard library is precompiled to bytecode in the image:
  the root filesystem is read-only, so Python could not cache it at run time.
- **JavaScript and TypeScript.** Only the `node` or `bun` binary is copied
  onto plain Alpine (npm, corepack and bunx are useless without network).
  Node detects ES module syntax by itself, so `main.js` may use `require` or
  `import`. Bun runs TypeScript, enums and namespaces included, by stripping
  the types: it does not type-check.
- **Bash** comes with GNU coreutils, findutils, grep, sed, gawk and bc rather
  than their BusyBox versions.

### Output streaming

When stdout is a pipe, as in the sandbox, most runtimes buffer it in 4–8 KiB
blocks: output would only show up at exit, out of order with stderr, and be
lost on a crash. Each image avoids that:

| language | how |
|----------|-----|
| python | `-u` (unbuffered) |
| ruby | `-r/opt/lightning/sync.rb` sets `$stdout.sync = true` |
| c, cpp | `linebuf.o`, linked into every program, makes stdout line-buffered from a constructor that runs before `main` (and before C++ static initializers); `std::cout` writes through it |
| php | the CLI flushes every write; `php/lightning.ini` sends errors to stderr, once |
| javascript, typescript, go, bash | write straight to the file descriptor |
| rust | `stdout` is line-buffered |
| java | `System.out` flushes on every line |

`test.sh` checks it for every language with a program that prints a line,
sleeps for a second and prints another.

## Build and test

```sh
runtimes/build.sh                     # every image, one at a time
runtimes/build.sh go rust             # some of them
PREFIX=ghcr.io/camilbenameur/lightning-runtime- TAG=1.0.0 runtimes/build.sh --push

runtimes/test.sh                      # every image
runtimes/test.sh python java          # some of them
```

Both scripts take `PREFIX` (default `lightning-runtime-`) and `TAG` (default
`latest`). `build.sh` is the same as
`docker build -t lightning-runtime-<id>:latest -f runtimes/<id>/Dockerfile runtimes/`.
Without a build cache (base images already pulled), all eleven take about
7 minutes on the machine above: 5–20 s each for most, 40 s for cpp (the
precompiled headers), 1–2.5 min for java (the training run), go (the standard
library) and rust (copying the toolchain). BuildKit is required (`COPY
--chmod`, `RUN --mount`); it is Docker's default builder.

`test.sh` runs the fixtures of `tests/<id>/` under the exact sandbox flags and
checks that:

| check | fixture | expects |
|-------|---------|---------|
| HELLO | `hello*.*` | `Hello, world!` and nothing else, exit 0, for every variant (C `<math.h>`, C++ `<bits/stdc++.h>` and `std::format`, Go standard library, Java classic and compact classes, ES modules, TypeScript enums) |
| STDIN | `echo.*` | copies `héllo ⚡⏎line2⏎` from stdin, byte for byte |
| EXIT | `exit.*` | `bye` on stderr, exit 3 |
| SYNTAX | `syntax.*` | non-zero exit, diagnostics on stderr, nothing on stdout |
| FS | `fs.*` | creating a file fails in `/etc` and the root, works in `/workspace` and `/tmp` |
| STREAM | `stream.*` | the second line arrives about a second after the first |

It ends with a summary table and exits non-zero if any check failed:

```
LANGUAGE    HELLO  STDIN  EXIT   SYNTAX  FS     STREAM    LATENCY     SIZE
bash        pass   pass   pass   pass    pass   pass       569 ms    14 MB
c           pass   pass   pass   pass    pass   pass       581 ms    99 MB
cpp         pass   pass   pass   pass    pass   pass       954 ms   337 MB
go          pass   pass   pass   pass    pass   pass      1299 ms   298 MB
java        pass   pass   pass   pass    pass   pass      1385 ms   179 MB
javascript  pass   pass   pass   pass    pass   pass       926 ms   142 MB
php         pass   pass   pass   pass    pass   pass       576 ms   111 MB
python      pass   pass   pass   pass    pass   pass       618 ms    66 MB
ruby        pass   pass   pass   pass    pass   pass       733 ms    77 MB
rust        pass   pass   pass   pass    pass   pass       862 ms   470 MB
typescript  pass   pass   pass   pass    pass   pass       513 ms    84 MB
all checks passed
```

A full run takes about 3 minutes.

The scripts pass `shellcheck`, the Dockerfiles `hadolint` (with
[.hadolint.yaml](.hadolint.yaml): Alpine only keeps the latest build of a
package, so `apk add` is not pinned; the base images are).

## Adding a language

1. Create `<id>/lang.sh`: set `SOURCE_FILE`, define `run()`, which must `exec`
   the program, and `build()` if there is a compile step (its output goes to
   stderr; a non-zero status stops the run with exit 1). Files go in
   `/workspace` or `/tmp`, the only writable places.
2. Create `<id>/Dockerfile` from an existing one: a pinned slim base image,
   the OCI labels, the `runner` user (uid/gid 10001) and `/workspace`,
   `common/run` as `/opt/lightning/run` and the `lang.sh` as
   `/opt/lightning/lang.sh`, `HOME=/tmp`, `WORKDIR /workspace`,
   `USER 10001:10001`. Bake in anything the toolchain would download or cache
   at run time, and make sure stdout is not block-buffered.
3. Add the fixtures to `tests/<id>/` and run `build.sh <id>` and
   `test.sh <id>` until every check passes.
4. Register the language in the API (`internal/language`, with its memory
   limit and compile allowance) and add its examples to the web app
   (`web/src/examples`).

## Security notes

The image is not the security boundary, the sandbox flags are (and, when
configured, a gVisor runtime). Within them, a program can do whatever an
unprivileged process can: run any toolchain in the image, use its CPU, memory,
process and file quotas, and read the image's files. Two details: the program
can read its own source and input back from the environment of PID 1 (tini),
which is harmless as they are its own; and the toolchains are those of
Alpine and the official images, updated by rebuilding the images.
