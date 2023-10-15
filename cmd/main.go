package main

import (
	"lightning/internal/rest"
	"log"
)

func main() {
	log.Println("Starting API server...")
	err := rest.StartAPIServer()

	if err != nil {
		log.Fatal(err)
	}
}
