#include <stdlib.h>
#include <unistd.h>
#include <sys/stat.h>
#include <stdio.h>
#include <errno.h>

void cleanup() {
    int ok = rmdir("/tmp/sub");
    printf("c %d %d\n", ok, errno);
    ok = rmdir("/tmp");
    printf("d %d %d\n", ok, errno);
}

int main() {
    atexit(cleanup);
    int ok = mkdir("/tmp", 0755);
    printf("a %d %d\n", ok, errno);
    ok = mkdir("/tmp/sub", 0755);
    printf("b %d %d\n", ok, errno);
    return 0;
}