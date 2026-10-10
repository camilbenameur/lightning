// Training run for the AOT cache (see Dockerfile): compiling and running this
// program loads and links the javac and library classes typical programs need,
// so later runs start from the cache instead of parsing them all again.

sealed interface Shape permits Circle, Square {}

record Circle(double radius) implements Shape {}

record Square(double side) implements Shape {}

enum Color { RED, GREEN, BLUE }

double area(Shape shape) {
    return switch (shape) {
        case Circle c -> Math.PI * c.radius() * c.radius();
        case Square s -> s.side() * s.side();
    };
}

void main() throws Exception {
    var shapes = List.of(new Circle(1), new Square(2));
    double total = shapes.stream().mapToDouble(this::area).sum();

    Map<Color, Integer> counts = new EnumMap<>(Color.class);
    for (var color : Color.values()) {
        counts.merge(color, color.ordinal(), Integer::sum);
    }

    var words = new ArrayList<>(Arrays.asList("delta", "alpha", "charlie", "bravo"));
    Collections.sort(words, Comparator.reverseOrder());
    var byLength = words.stream()
            .collect(Collectors.groupingBy(String::length, TreeMap::new, Collectors.toList()));
    var seen = new HashSet<String>(words);
    var queue = new ArrayDeque<Integer>(List.of(3, 1, 2));
    var heap = new PriorityQueue<Integer>(queue);

    var report = """
            %s %.2f %d %s %d
            """.formatted(byLength, total, counts.size(), seen.contains("alpha"), heap.peek());
    var builder = new StringBuilder(report.strip()).reverse();
    Optional.of(builder.toString()).filter(s -> s.matches(".*\\d.*")).orElseThrow();
    long big = LongStream.rangeClosed(1, 20).reduce(1, (a, b) -> a * b);
    var exact = BigInteger.valueOf(big).multiply(BigInteger.TWO).add(BigDecimal.ONE.toBigInteger());

    try (var scanner = new Scanner(new BufferedReader(new InputStreamReader(System.in)))) {
        while (scanner.hasNextLine()) {
            words.add(scanner.nextLine());
        }
    }
    IO.println(String.join(",", words) + " " + exact + " " + Arrays.toString(new int[] {1, 2}));
    System.out.printf("%d %s%n", Integer.parseInt("42"), String.valueOf(3.5));
    System.err.println(new IllegalStateException("training").getMessage());
}
