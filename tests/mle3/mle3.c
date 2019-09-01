#include <stdio.h>
#include <unistd.h>
#include <stdlib.h>
#include <string.h>

int main() {
    int *c, i;

    for(i = 0; i < 40000; i++) {
        c = (int *)malloc(1024 * 4);
        memset(c, sizeof(c), 1);
   }
    return 0;
}
