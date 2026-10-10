use std::fs;

fn main() {
    for path in ["/etc/lightning", "/workspace/../lightning", "/workspace/lightning", "/tmp/lightning"] {
        let result = if fs::write(path, "").is_ok() { "ok" } else { "error" };
        println!("{path}: {result}");
    }
}
