use std::io::{self, BufRead};

fn main() {
    for line in io::stdin().lock().lines() {
        println!("{}", line.expect("stdin is valid UTF-8"));
    }
}
