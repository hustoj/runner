#define _GNU_SOURCE

#include <stdio.h>
#include <string.h>
#include <sys/resource.h>
#include <unistd.h>

static char output_buffer[65536];

int main(void)
{
    struct rlimit limit;
    struct rlimit raised;
    size_t written = 0;
    const size_t target = 1536 * 1024;

    memset(output_buffer, 'A', sizeof(output_buffer));

    if (prlimit(0, RLIMIT_FSIZE, NULL, &limit) != 0) {
        perror("prlimit get");
        return 2;
    }

    raised.rlim_cur = limit.rlim_max;
    raised.rlim_max = limit.rlim_max;
    if (prlimit(0, RLIMIT_FSIZE, &raised, NULL) != 0) {
        perror("prlimit set");
        return 3;
    }

    while (written < target) {
        ssize_t n = write(STDOUT_FILENO, output_buffer, sizeof(output_buffer));
        if (n < 0) {
            perror("write");
            return 4;
        }
        written += (size_t)n;
    }

    return 0;
}
