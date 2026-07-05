#define _GNU_SOURCE

#include <sys/syscall.h>
#include <unistd.h>

int main(void)
{
    syscall(SYS_getpid);
    return 0;
}
