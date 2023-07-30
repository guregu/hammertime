#include <time.h>
#include <stdio.h>

int main(void) {
    struct timespec ts;
    clock_gettime(CLOCK_MONOTONIC, &ts);
    printf("%lld %ld\n", ts.tv_sec, ts.tv_nsec);
    return 0;
}
