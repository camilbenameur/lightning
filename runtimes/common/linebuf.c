/*
 * Linked into every C and C++ program. stdout is fully buffered when it is a
 * pipe, so output would only show up at exit (or never, after a crash); make it
 * line-buffered so it streams live and stays in order with stderr. Priority 101
 * runs this before any default-priority constructor, e.g. C++ static objects.
 */
#include <stdio.h>

__attribute__((constructor(101))) static void lightning_line_buffer_stdout(void) {
    setvbuf(stdout, NULL, _IOLBF, BUFSIZ);
}
