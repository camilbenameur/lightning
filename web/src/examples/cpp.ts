import { NAMES, source } from './shared.ts';
import type { Example } from './types.ts';

export const cpp: Example[] = [
  {
    id: 'hello',
    title: 'Hello, world!',
    code: source`
#include <iostream>

int main() {
    std::cout << "Hello, world!\n";
}
`,
  },
  {
    id: 'stdin',
    title: 'Read from stdin',
    code: source`
#include <iostream>
#include <string>

int main() {
    std::string name;
    while (std::getline(std::cin, name)) {
        if (!name.empty()) {
            std::cout << "Hello, " << name << "!\n";
        }
    }
}
`,
    stdin: NAMES,
  },
  {
    id: 'fibonacci',
    title: 'Fibonacci',
    code: source`
#include <array>
#include <cstddef>
#include <iostream>

// Evaluated by the compiler: the table is baked into the binary.
constexpr auto make_fibonacci() {
    std::array<unsigned long long, 20> table{};
    table[1] = 1;
    for (std::size_t i = 2; i < table.size(); ++i) {
        table[i] = table[i - 1] + table[i - 2];
    }
    return table;
}

int main() {
    constexpr auto fib = make_fibonacci();
    static_assert(fib[10] == 55);
    for (std::size_t i = 0; i < fib.size(); ++i) {
        std::cout << "fib(" << i << ") = " << fib[i] << '\n';
    }
}
`,
  },
  {
    id: 'showcase',
    title: 'Ranges & lambdas',
    code: source`
#include <algorithm>
#include <functional>
#include <iostream>
#include <numeric>
#include <ranges>
#include <vector>

namespace views = std::views;

int main() {
    std::vector<int> nums(10);
    std::iota(nums.begin(), nums.end(), 1);

    auto is_even = [](int n) { return n % 2 == 0; };
    auto square = [](int n) { return n * n; };

    std::cout << "even squares:";
    for (int n : nums | views::filter(is_even) | views::transform(square)) {
        std::cout << ' ' << n;
    }
    std::cout << "\nevens: " << std::ranges::count_if(nums, is_even);
    std::cout << "\nsum: " << std::accumulate(nums.begin(), nums.end(), 0) << '\n';

    int threshold = 7;
    auto above = [threshold](int n) { return n > threshold; };
    std::ranges::sort(nums, std::greater{});
    std::cout << "above " << threshold << ':';
    for (int n : nums | views::take_while(above)) {
        std::cout << ' ' << n;
    }
    std::cout << '\n';
}
`,
  },
];
