package main

import (
	"log"

	"github.com/Chroq/zombie-horde/internal/server"
)

func main() {
	if err := server.Run(":8080", "."); err != nil {
		log.Fatalf("Erreur serveur: %v", err)
	}
}
