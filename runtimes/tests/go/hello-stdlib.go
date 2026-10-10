package main

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"
)

func main() {
	words := []string{"world!", "Hello,"}
	slices.Reverse(words)
	data, _ := json.Marshal(strings.Join(words, " "))
	var greeting string
	_ = json.Unmarshal(data, &greeting)
	fmt.Println(greeting)
}
