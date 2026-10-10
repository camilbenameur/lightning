package main

import (
	"fmt"
	"time"
)

func main() {
	fmt.Println("tick")
	time.Sleep(time.Second)
	fmt.Println("tock")
}
