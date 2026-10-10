#include <stdio.h>

int main(void) {
    char line[256];
    while (fgets(line, sizeof line, stdin)) {
        fputs(line, stdout);
    }
    return 0;
}
