#include <stdio.h>
#include <string.h>

int foo(int i){
    int a[10000000];
    memset(a, 0x1, sizeof(a));
    if (i>1000) return 0;
    return foo(i+1);
}

int main() {
    foo(1);
    return 0;
}
