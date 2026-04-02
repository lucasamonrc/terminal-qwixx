package main

import (
	"flag"
	"log"
	"os"

	"github.com/lucasacastro/qwixx/server"
)

func main() {
	host := flag.String("host", "0.0.0.0", "Host to bind to")
	port := flag.Int("port", 2222, "Port to listen on")
	flag.Parse()

	// Ensure .ssh directory exists for host key
	if err := os.MkdirAll(".ssh", 0700); err != nil {
		log.Fatalf("Failed to create .ssh directory: %v", err)
	}

	srv := server.New(*host, *port)
	if err := srv.Start(); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}
