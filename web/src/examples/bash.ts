import { NAMES, source } from './shared.ts';
import type { Example } from './types.ts';

export const bash: Example[] = [
  {
    id: 'hello',
    title: 'Hello, world!',
    code: source`
echo "Hello, world!"
`,
  },
  {
    id: 'stdin',
    title: 'Read from stdin',
    code: source`
while IFS= read -r name || [[ -n $name ]]; do
  if [[ -n $name ]]; then
    echo "Hello, $name!"
  fi
done
`,
    stdin: NAMES,
  },
  {
    id: 'fizzbuzz',
    title: 'FizzBuzz',
    code: source`
for ((i = 1; i <= 20; i++)); do
  out=""
  ((i % 3 == 0)) && out+="Fizz"
  ((i % 5 == 0)) && out+="Buzz"
  echo "\${out:-$i}"
done
`,
  },
  {
    id: 'showcase',
    title: 'Arrays & parameter expansion',
    code: source`
fruits=(apple banana cherry date)

shout() {
  local word=$1
  echo "\${word^^}!"
}

for fruit in "\${fruits[@]}"; do
  shout "$fruit"
done

echo "count: \${#fruits[@]}"
echo "last: \${fruits[-1]}"
echo "middle: \${fruits[*]:1:2}"

path="/tmp/archive.tar.gz"
echo "file: \${path##*/}"
echo "stem: \${path%%.*}"
echo "ext: \${path#*.}"
echo "renamed: \${path/tar.gz/zip}"
echo "greeting: \${nickname:-stranger}"
`,
  },
];
