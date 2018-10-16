#include <stdio.h>
#include <unistd.h>

int main(void) {
    int a, b;
    scanf("%d%d", &a, &b);
    printf("%d", a + b);
    sleep(10);
    return 0;
}
