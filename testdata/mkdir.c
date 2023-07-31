#include <stdlib.h>
#include <unistd.h>
#include <sys/stat.h>
#include <stdio.h>
#include <errno.h>

void cleanup() {
    int ok = rmdir("/tmp");
    printf("%d %d\n", ok, errno);
}

int main() {
    atexit(cleanup);
    int ok = mkdir("/tmp", 0755);
    printf("%d %d\n", ok, errno);
    return 0;
}