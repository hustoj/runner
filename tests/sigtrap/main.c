#include <sys/socket.h>
#include <unistd.h>

int main(void)
{
    __asm__ volatile("int3");

    int socket_fd = socket(AF_INET, SOCK_STREAM, 0);
    if (socket_fd >= 0) {
        close(socket_fd);
    }

    return 0;
}
