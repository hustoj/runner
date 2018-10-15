#include "cfork.h"

int fork_and_return(){
    pid_t pid = vfork();
    if (pid < 0){
        printf("pid error.... %d\n", pid);
    }else if (pid == 0){
        freopen("user.in", "r", stdin);
        freopen("user.out", "w", stdout);
        freopen("user.err", "w", stderr);
        alarm(1);
        ptrace(PTRACE_TRACEME, 0, NULL, NULL);
        execl("./Main", NULL);
    }else{
        return pid;
    }
}

