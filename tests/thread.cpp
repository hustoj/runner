#include <thread>

void new_thread()
{
    int total = 0;
    while (true)
    {
        std::thread *th = new std::thread(new_thread);
        total++;
        if (total > 10) {
            break;
        }
    }
}

int main()
{
    new_thread();
    return 0;
}