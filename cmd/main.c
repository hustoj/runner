#include <stdio.h>
#include <unistd.h>
#include <time.h>
#include <stdlib.h>
#include <string.h>

//int d[1024 * 1024];

int main(void) {
    int a, b;
    int i;
    int *c;
//    int d[1024 * 1024];
    for(i = 0; i < 100; i++) {
        c = (int *)malloc(1024 * 128);
        memset(c, 1024 * 128, 1);
//        sleep(1);
   }
    return 0;
}
