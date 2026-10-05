package main

import (
	"flag"
	"fmt"
	"log"

	"github.com/Chroq/zombie-horde/internal/server"
)

func main() {
	port := flag.Int("port", 8080, "Port d'écoute du serveur HTTP et WebSocket")
	staticDir := flag.String("static", ".", "Chemin vers le répertoire contenant index.html")
	flag.Parse()

	addr := fmt.Sprintf(":%d", *port)
	if err := server.Run(addr, *staticDir); err != nil {
		log.Fatalf("Erreur serveur: %v", err)
	}
}
