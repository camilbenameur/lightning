#include <chrono>
#include <iostream>
#include <thread>

int main() {
    std::cout << "tick\n";
    std::this_thread::sleep_for(std::chrono::seconds(1));
    std::cout << "tock\n";
}
