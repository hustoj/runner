#include <stdio.h>

int main() {
    int i,k;
    for(i = 0; i < 1000000; i++) {
        for (k = 0; k < 1000000; k++) {
            i+k;
        }
    }
    return 0;
}
