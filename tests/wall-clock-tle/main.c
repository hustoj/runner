#include <signal.h>
#include <unistd.h>

static void on_alarm(int sig) {
    (void)sig;
}

int main(void) {
    signal(SIGALRM, on_alarm);
    for (;;) {
        pause();
    }
}
