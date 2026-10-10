#include <math.h>
#include <stdio.h>

int main(void) {
    if (fabs(cos(M_PI) + 1.0) < 1e-12) {
        puts("Hello, world!");
    }
    return 0;
}
