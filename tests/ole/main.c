#include <stdio.h>

int main() {
    int a, b;
    while(1) printf("Hello World!\n");
    while(EOF!=scanf("%d%d", &a, &b))
        printf("%d\n", a + b);
    return 0;
}
