import { NAMES, source } from './shared.ts';
import type { Example } from './types.ts';

export const rust: Example[] = [
  {
    id: 'hello',
    title: 'Hello, world!',
    code: source`
fn main() {
    println!("Hello, world!");
}
`,
  },
  {
    id: 'stdin',
    title: 'Read from stdin',
    code: source`
use std::io::{self, BufRead};

fn main() {
    for line in io::stdin().lock().lines().map_while(Result::ok) {
        let name = line.trim();
        if !name.is_empty() {
            println!("Hello, {name}!");
        }
    }
}
`,
    stdin: NAMES,
  },
  {
    id: 'fizzbuzz',
    title: 'FizzBuzz',
    code: source`
fn main() {
    for n in 1..=20 {
        match (n % 3, n % 5) {
            (0, 0) => println!("FizzBuzz"),
            (0, _) => println!("Fizz"),
            (_, 0) => println!("Buzz"),
            _ => println!("{n}"),
        }
    }
}
`,
  },
  {
    id: 'showcase',
    title: 'Enums & pattern matching',
    code: source`
use std::f64::consts::PI;

#[derive(Debug)]
enum Shape {
    Circle(f64),
    Rect { width: f64, height: f64 },
    Triangle(f64, f64, f64),
}

impl Shape {
    fn area(&self) -> f64 {
        match *self {
            Shape::Circle(r) => PI * r * r,
            Shape::Rect { width, height } => width * height,
            Shape::Triangle(a, b, c) => {
                let s = (a + b + c) / 2.0;
                (s * (s - a) * (s - b) * (s - c)).sqrt()
            }
        }
    }
}

fn main() {
    let shapes = [
        Shape::Circle(1.5),
        Shape::Rect {
            width: 3.0,
            height: 4.0,
        },
        Shape::Triangle(3.0, 4.0, 5.0),
    ];

    for shape in &shapes {
        println!("{shape:?} has area {:.2}", shape.area());
    }

    let total: f64 = shapes.iter().map(Shape::area).sum();
    let largest = shapes.iter().max_by(|a, b| a.area().total_cmp(&b.area()));
    println!("total area: {total:.2}, largest: {largest:?}");
}
`,
  },
];
