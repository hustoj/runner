#include <stdio.h>

int main() {
    int a, b;
    int c[100];
    for (a=0;a<1000;a-=10)
        c[a]=c[a-1]+a;
    return 0;
}