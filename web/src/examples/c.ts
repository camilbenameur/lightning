import { NAMES, source } from './shared.ts';
import type { Example } from './types.ts';

export const c: Example[] = [
  {
    id: 'hello',
    title: 'Hello, world!',
    code: source`
#include <stdio.h>

int main(void) {
    puts("Hello, world!");
    return 0;
}
`,
  },
  {
    id: 'stdin',
    title: 'Read from stdin',
    code: source`
#include <stdio.h>
#include <string.h>

int main(void) {
    char name[128];
    while (fgets(name, sizeof name, stdin)) {
        name[strcspn(name, "\n")] = '\0';
        if (name[0] != '\0') {
            printf("Hello, %s!\n", name);
        }
    }
    return 0;
}
`,
    stdin: NAMES,
  },
  {
    id: 'fibonacci',
    title: 'Fibonacci',
    code: source`
#include <stdio.h>

int main(void) {
    unsigned long long a = 0, b = 1;
    for (int i = 0; i < 20; i++) {
        printf("fib(%d) = %llu\n", i, a);
        unsigned long long next = a + b;
        a = b;
        b = next;
    }
    return 0;
}
`,
  },
  {
    id: 'showcase',
    title: 'Structs, pointers & qsort',
    code: source`
#include <stdio.h>
#include <stdlib.h>

typedef struct {
    const char *name;
    int score;
} Player;

static int by_score_desc(const void *a, const void *b) {
    const Player *x = a, *y = b;
    return (y->score > x->score) - (y->score < x->score);
}

static void print_all(const Player *first, const Player *end) {
    for (const Player *p = first; p < end; p++) {
        printf("%-6s %3d\n", p->name, p->score);
    }
}

int main(void) {
    Player players[] = {
        {"Ada", 91}, {"Linus", 78}, {"Grace", 95}, {"Dennis", 85},
    };
    size_t count = sizeof players / sizeof players[0];

    qsort(players, count, sizeof players[0], by_score_desc);
    print_all(players, players + count);

    Player *best = &players[0];
    best->score += 5;
    printf("winner: %s with %d\n", best->name, best->score);
    return 0;
}
`,
  },
];
