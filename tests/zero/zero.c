#include <stdio.h>
#include <string.h>

int main() {
    int a, b;
    b = 0;
    for (a=0;a<1000;a++)
        b += 100/(500-a);
    return 0;
}
