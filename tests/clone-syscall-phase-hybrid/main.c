#define _GNU_SOURCE

#include <signal.h>
#include <stdio.h>
#include <sys/syscall.h>
#include <unistd.h>

int main(void)
{
    long pid = syscall(SYS_clone, SIGCHLD, 0, 0, 0, 0);
    if (pid == 0) {
        _exit(0);
    }
    if (pid < 0) {
        return 2;
    }

    rename("sentinel.source", "sentinel.dest");
    return 0;
}
