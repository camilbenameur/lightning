#include <fstream>
#include <iostream>

int main() {
    for (const char *path : {"/etc/lightning", "/workspace/../lightning", "/workspace/lightning", "/tmp/lightning"}) {
        std::ofstream file(path);
        std::cout << path << ": " << (file ? "ok" : "error") << '\n';
    }
}
