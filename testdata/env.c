#include <stdlib.h>
#include <stdio.h>

int main(void) {
    char *got = getenv("TEST");
    printf("%s\n", got);
    return 0;
}
