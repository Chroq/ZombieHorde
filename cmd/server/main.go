package main

import (
	"flag"
	"fmt"
	"log"
	"runtime"

	"github.com/Chroq/zombie-horde/internal/server"

	"net/http"
	_ "net/http/pprof"
)

func main() {
	runtime.SetMutexProfileFraction(1) // Échantillonne 100% des événements de contention de verrous (pprof -mutex)
	runtime.SetBlockProfileRate(1)      // Échantillonne les délais de blocage d'ordonnancement

	go func() { _ = http.ListenAndServe("localhost:6060", nil) }()

	port := flag.Int("port", 8080, "Port d'écoute du serveur HTTP et WebSocket")
	staticDir := flag.String("static", ".", "Chemin vers le répertoire contenant index.html")
	flag.Parse()

	addr := fmt.Sprintf(":%d", *port)
	if err := server.Run(addr, *staticDir); err != nil {
		log.Fatalf("Erreur serveur: %v", err)
	}
}
