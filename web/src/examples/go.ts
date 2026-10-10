import { NAMES, source } from './shared.ts';
import type { Example } from './types.ts';

export const go: Example[] = [
  {
    id: 'hello',
    title: 'Hello, world!',
    code: source`
package main

import "fmt"

func main() {
	fmt.Println("Hello, world!")
}
`,
  },
  {
    id: 'stdin',
    title: 'Read from stdin',
    code: source`
package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

func main() {
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		if name := strings.TrimSpace(scanner.Text()); name != "" {
			fmt.Printf("Hello, %s!\n", name)
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
package main

import "fmt"

func fibonacci() func() int {
	a, b := 0, 1
	return func() int {
		n := a
		a, b = b, a+b
		return n
	}
}

func main() {
	next := fibonacci()
	for i := range 20 {
		fmt.Printf("fib(%d) = %d\n", i, next())
	}
}
`,
  },
  {
    id: 'showcase',
    title: 'Goroutines & channels',
    code: source`
package main

import (
	"fmt"
	"sync"
)

type result struct{ n, square int }

func main() {
	jobs := make(chan int)
	results := make(chan result)

	var workers sync.WaitGroup
	for range 3 {
		workers.Go(func() {
			for n := range jobs {
				results <- result{n, n * n}
			}
		})
	}

	go func() {
		for n := 1; n <= 8; n++ {
			jobs <- n
		}
		close(jobs)
	}()
	go func() {
		workers.Wait()
		close(results)
	}()

	squares := make([]int, 8)
	for r := range results {
		squares[r.n-1] = r.square
	}
	fmt.Println(squares)
}
`,
  },
];
