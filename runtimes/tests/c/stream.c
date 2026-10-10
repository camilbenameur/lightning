#include <stdio.h>
#include <unistd.h>

int main(void) {
    puts("tick");
    sleep(1);
    puts("tock");
    return 0;
}
