void main() {
    for (var path : List.of("/etc/lightning", "/workspace/../lightning", "/workspace/lightning", "/tmp/lightning")) {
        try {
            Files.writeString(Path.of(path), "");
            IO.println(path + ": ok");
        } catch (IOException e) {
            IO.println(path + ": error");
        }
    }
}
