package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/Chroq/zombie-horde/internal/simulation"
	"golang.org/x/net/websocket"
)

// Server gère le serveur HTTP de télémétrie et le serveur WebSocket.
type Server struct {
	addr      string
	staticDir string
}

// New crée une nouvelle instance de Server.
func New(addr string, staticDir string) *Server {
	if staticDir == "" {
		staticDir = "."
	}
	return &Server{
		addr:      addr,
		staticDir: staticDir,
	}
}

// Run démarre le serveur HTTP et écoute les connexions.
func Run(addr string, staticDir string) error {
	s := New(addr, staticDir)
	return s.Start()
}

// Start configure les routes et démarre l'écoute HTTP.
func (s *Server) Start() error {
	mux := http.NewServeMux()

	// Résolution du répertoire statique (recherche d'index.html)
	resolvedDir := s.resolveStaticDir()
	mux.Handle("/", http.FileServer(http.Dir(resolvedDir)))
	mux.Handle("/ws", websocket.Handler(s.handleWebSocket))

	fmt.Printf("Moteur d'épidémie ZombieHorde disponible sur http://localhost%s\n", s.addr)
	return http.ListenAndServe(s.addr, mux)
}

func (s *Server) resolveStaticDir() string {
	// 1. Dossier spécifié
	if _, err := os.Stat(filepath.Join(s.staticDir, "index.html")); err == nil {
		return s.staticDir
	}
	// 2. Dossier parent (si lancé depuis cmd/server)
	if _, err := os.Stat("../index.html"); err == nil {
		return ".."
	}
	if _, err := os.Stat("../../index.html"); err == nil {
		return "../.."
	}
	return s.staticDir
}

func (s *Server) handleWebSocket(ws *websocket.Conn) {
	defer ws.Close()

	engine := simulation.NewEngine(simulation.MasterSeed)

	ticker := time.NewTicker(time.Second / time.Duration(simulation.TargetFPS))
	defer ticker.Stop()

	frames := 0
	lastCheck := time.Now()
	currentTPS := 0

	for range ticker.C {
		engine.Update()

		frames++
		if time.Since(lastCheck) >= time.Second {
			currentTPS = frames
			frames = 0
			lastCheck = time.Now()
		}

		payload := engine.BuildFramePayload(currentTPS)

		data, err := json.Marshal(payload)
		if err != nil {
			break
		}

		if err := websocket.Message.Send(ws, string(data)); err != nil {
			break
		}
	}
}
