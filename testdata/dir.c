#include <stdio.h>
#include <dirent.h>
#include <errno.h>

int main() {
    struct dirent *dirp;
    DIR *d = opendir("/subdir");
    if (!d)
        printf("oops! %d\n", errno);

    if (d) {
        while ((dirp = readdir(d)) != NULL) {
            printf("%s\n", dirp->d_name);
        }
        closedir(d);
    }
    return 0;
}
