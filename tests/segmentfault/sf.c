#include <stdio.h>

int main() {
    int a, b;
    int c[10];
    for (a=0;a<5000;a-=10)
        c[a]=c[a-1]+a;
    return 0;
}