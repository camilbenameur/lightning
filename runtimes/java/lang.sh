# shellcheck shell=sh disable=SC2034 # sourced by common/run, which uses SOURCE_FILE
SOURCE_FILE=Main.java
# The source launcher compiles Main.java in memory and runs it, so classic
# `public class Main` and compact `void main()` programs both work. Tuned for
# short programs on one CPU: serial GC, and only the C1 JIT, which gets javac
# up to speed about 30% sooner than tiered C1 + C2 (at the cost of peak speed
# in long CPU-bound loops). -UsePerfData skips the hsperfdata file, and JVM
# warnings go to stderr instead of stdout. The class path is an empty directory
# so that it matches the training run that built the AOT cache (see Dockerfile).
java_main() {
  exec java -XX:+UseSerialGC -XX:TieredStopAtLevel=1 -XX:-UsePerfData \
    -Xlog:disable -Xlog:all=warning:stderr -cp /opt/lightning/classes "$@" Main.java
}
run() { java_main -XX:AOTCache=/opt/lightning/java.aot; }
