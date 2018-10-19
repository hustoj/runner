#include <stdio.h>
#include <unistd.h>
#include <stdlib.h>
#include <string.h>

int main() {
    int *c, i;

    for(i = 0; i < 100; i++) {
        c = (int *)malloc(1024 * 1024);
        memset(c, 1024 * 1024, 1);
        sleep(1);
   }
    return 0;
}
