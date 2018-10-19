#include <stdio.h>

int main() {
    int i,k;
    for(i = 0; i < 100000; i++) {
        for (k = 0; k < 10000; k++) {
            i+k;
        }
    }
    return 0;
}
