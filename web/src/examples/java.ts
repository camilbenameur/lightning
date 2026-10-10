import { NAMES, source } from './shared.ts';
import type { Example } from './types.ts';

export const java: Example[] = [
  {
    id: 'hello',
    title: 'Hello, world!',
    code: source`
public class Main {
    public static void main(String[] args) {
        System.out.println("Hello, world!");
    }
}
`,
  },
  {
    id: 'stdin',
    title: 'Read from stdin',
    code: source`
import java.util.Scanner;

public class Main {
    public static void main(String[] args) {
        Scanner in = new Scanner(System.in);
        while (in.hasNextLine()) {
            String name = in.nextLine().strip();
            if (!name.isEmpty()) {
                System.out.println("Hello, " + name + "!");
            }
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
public class Main {
    public static void main(String[] args) {
        for (int i = 1; i <= 20; i++) {
            String line = switch (i % 15) {
                case 0 -> "FizzBuzz";
                case 3, 6, 9, 12 -> "Fizz";
                case 5, 10 -> "Buzz";
                default -> String.valueOf(i);
            };
            System.out.println(line);
        }
    }
}
`,
  },
  {
    id: 'showcase',
    title: 'Records, streams & pattern matching',
    code: source`
import java.util.List;

sealed interface Shape {
    record Circle(double radius) implements Shape {}

    record Rect(double width, double height) implements Shape {}
}

double area(Shape shape) {
    return switch (shape) {
        case Shape.Circle(double r) -> Math.PI * r * r;
        case Shape.Rect(double w, double h) -> w * h;
    };
}

void main() {
    List<Shape> shapes = List.of(
            new Shape.Circle(1), new Shape.Rect(3, 4), new Shape.Circle(2));

    shapes.forEach(s -> IO.println("%s -> %.2f".formatted(s, area(s))));

    double total = shapes.stream().mapToDouble(this::area).sum();
    long circles = shapes.stream().filter(s -> s instanceof Shape.Circle).count();
    IO.println("total area: %.2f (%d circles)".formatted(total, circles));
}
`,
  },
];
