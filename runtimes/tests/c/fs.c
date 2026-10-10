#include <stdio.h>

int main(void) {
    const char *paths[] = {"/etc/lightning", "/workspace/../lightning", "/workspace/lightning", "/tmp/lightning"};
    for (size_t i = 0; i < sizeof paths / sizeof *paths; i++) {
        FILE *file = fopen(paths[i], "w");
        printf("%s: %s\n", paths[i], file ? "ok" : "error");
        if (file) {
            fclose(file);
        }
    }
    return 0;
}
