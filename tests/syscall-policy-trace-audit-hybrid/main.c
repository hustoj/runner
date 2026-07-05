#define _GNU_SOURCE

#include <sys/syscall.h>
#include <unistd.h>

int main(void)
{
    long pid = syscall(SYS_getpid);
    long ppid = syscall(SYS_getppid);
    if (pid <= 0 || ppid <= 0) {
        return 1;
    }
    return 0;
}
