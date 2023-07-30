#include <stdio.h>

int main(int argc, char **argv) {
    FILE *f = fopen("test.txt", "r");
    if (!f)
        return 1;

    int c;
    while ((c = fgetc(f)) != EOF)
        putchar(c);

    fclose(f);
    return 0;
}
