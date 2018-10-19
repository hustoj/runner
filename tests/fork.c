#include <stdio.h>
#include <stdlib.h>
#include <time.h>
#include <unistd.h>

int main()
{
    if (!fork()) // child
    {
        sleep(10);
//        while (1)
//        {
//            fork();
//        }
    }
    puts("hello, world");
    sleep(10);
    return 233;
}