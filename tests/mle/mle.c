#include <stdio.h>
#include <stdlib.h>
#include <string.h>

int main() {
    int d[1024 * 1024];
    memset(d, 1024 * 1024, 1);
    int i;
    for(i = 0;i < 1024 * 1024;i++) {
        d[i] = 1;
    }

    return 0;
}
