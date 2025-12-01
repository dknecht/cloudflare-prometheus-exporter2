package main

import (
	"log"

	"github.com/lablabs/cloudflare-exporter/internal/cli"
)

func main() {
	if err := cli.Execute(); err != nil {
		log.Fatalf("Application failed: %v", err)
	}
}
