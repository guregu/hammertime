#include <stdlib.h>
#include <stdio.h>

int main(int argc, char **argv) {
    if (argc < 2)
        return -1;
    char *who = argv[1];
    char *greet = getenv("GREET");
    if (!greet)
        greet = "greetings";
    printf("%s %s\n", greet, who);
    return 0;
}
