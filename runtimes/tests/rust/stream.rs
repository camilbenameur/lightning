use std::{thread, time::Duration};

fn main() {
    println!("tick");
    thread::sleep(Duration::from_secs(1));
    println!("tock");
}
