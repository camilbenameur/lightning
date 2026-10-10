package main

import (
	"fmt"
	"os"
)

func main() {
	for _, path := range []string{"/etc/lightning", "/workspace/../lightning", "/workspace/lightning", "/tmp/lightning"} {
		result := "ok"
		if err := os.WriteFile(path, nil, 0o644); err != nil {
			result = "error"
		}
		fmt.Printf("%s: %s\n", path, result)
	}
}
