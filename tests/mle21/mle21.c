#include <stdio.h>
#include <time.h>
#include <stdlib.h>
#include <string.h>

int d[1024 * 1024] = {0, 1};

int main() {
    memset(d, 1024 * 1024, 1);
    int i;
    for(i = 0;i < 1024 * 1024;i++) {
        d[i] = 1;
    }

    return 0;
}
