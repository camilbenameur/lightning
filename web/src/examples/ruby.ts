import { NAMES, source } from './shared.ts';
import type { Example } from './types.ts';

export const ruby: Example[] = [
  {
    id: 'hello',
    title: 'Hello, world!',
    code: source`
puts 'Hello, world!'
`,
  },
  {
    id: 'stdin',
    title: 'Read from stdin',
    code: source`
$stdin.each_line do |line|
  name = line.strip
  puts "Hello, #{name}!" unless name.empty?
end
`,
    stdin: NAMES,
  },
  {
    id: 'fibonacci',
    title: 'Fibonacci',
    code: source`
fibonacci = Enumerator.new do |yielder|
  a, b = 0, 1
  loop do
    yielder << a
    a, b = b, a + b
  end
end

puts fibonacci.take(20).join(' ')
p fibonacci.lazy.select(&:even?).first(5)
`,
  },
  {
    id: 'showcase',
    title: 'Blocks & Enumerable',
    code: source`
class Countdown
  include Enumerable

  def initialize(from)
    @from = from
  end

  # Implement each with a block and Enumerable supplies the rest.
  def each
    @from.downto(1) { |n| yield n }
  end
end

countdown = Countdown.new(6)
p countdown.map { |n| n * n }
p countdown.select(&:even?)
p countdown.each_slice(4).to_a
p countdown.reduce(:*)
p countdown.min_by { |n| (n - 4).abs }
p countdown.partition(&:odd?)
p countdown.lazy.map { |n| n * 3 }.first(2)
`,
  },
];
