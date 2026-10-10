import { NAMES, source } from './shared.ts';
import type { Example } from './types.ts';

export const php: Example[] = [
  {
    id: 'hello',
    title: 'Hello, world!',
    code: source`
<?php

echo "Hello, world!\n";
`,
  },
  {
    id: 'stdin',
    title: 'Read from stdin',
    code: source`
<?php

$names = file('php://stdin', FILE_IGNORE_NEW_LINES | FILE_SKIP_EMPTY_LINES);
foreach ($names as $name) {
    echo "Hello, $name!\n";
}
`,
    stdin: NAMES,
  },
  {
    id: 'fibonacci',
    title: 'Fibonacci',
    code: source`
<?php

function fibonacci(): Generator
{
    [$a, $b] = [0, 1];
    while (true) {
        yield $a;
        [$a, $b] = [$b, $a + $b];
    }
}

foreach (fibonacci() as $i => $n) {
    if ($i === 20) {
        break;
    }
    echo "fib($i) = $n\n";
}
`,
  },
  {
    id: 'showcase',
    title: 'match & arrow functions',
    code: source`
<?php

declare(strict_types=1);

$grade = fn(int $score): string => match (true) {
    $score >= 90 => 'A',
    $score >= 80 => 'B',
    default => 'C',
};

$scores = ['Ada' => 93, 'Linus' => 78, 'Grace' => 85];

echo json_encode(array_map($grade, $scores)), "\n";
$passing = array_filter($scores, fn(int $s) => $s >= 80);
echo implode(', ', array_keys($passing)), "\n";

$bonus = 5; // arrow functions capture the enclosing scope by value
echo implode(' ', array_map(fn(int $s) => $s + $bonus, $scores)), "\n";

arsort($scores);
echo 'top: ', array_key_first($scores), "\n";
echo 'average: ', round(array_sum($scores) / count($scores), 1), "\n";
`,
  },
];
