package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/Chroq/zombie-horde/internal/simulation"
	"golang.org/x/net/websocket"
)

// BufferPool recycle les tampons mémoire réseau bytes.Buffer pour éviter les allocations GC répétées.
var BufferPool = sync.Pool{
	New: func() any {
		return bytes.NewBuffer(make([]byte, 0, 256*1024))
	},
}

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

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	engine := simulation.NewEngine(simulation.MasterSeed)
	defer engine.Close()

	useBinary := true
	if ws.Request() != nil && ws.Request().URL.Query().Get("format") == "json" {
		useBinary = false
	}

	// Pour le streaming binaire, émettre la frame d'initialisation statique (obstacles, sas, monde)
	if useBinary {
		ws.PayloadType = websocket.BinaryFrame
		initBuf := BufferPool.Get().(*bytes.Buffer)
		engine.EncodeInitFrame(initBuf)
		if err := websocket.Message.Send(ws, initBuf.Bytes()); err != nil {
			BufferPool.Put(initBuf)
			return
		}
		BufferPool.Put(initBuf)
	}

	ticker := time.NewTicker(time.Second / time.Duration(simulation.TargetFPS))
	defer ticker.Stop()

	frames := 0
	lastCheck := time.Now()
	currentTPS := 0

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// Coordonner la fin du calcul de frame avec sync.WaitGroup avant la diffusion réseau
			var wg sync.WaitGroup
			wg.Add(1)
			go func() {
				defer wg.Done()
				engine.Update()
			}()
			wg.Wait()

			frames++
			if time.Since(lastCheck) >= time.Second {
				currentTPS = frames
				frames = 0
				lastCheck = time.Now()
			}

			buf := BufferPool.Get().(*bytes.Buffer)
			buf.Reset()

			var isFinished bool

			if useBinary {
				// Streaming binaire direct (byte-packing compact, 5 octets par agent)
				ws.PayloadType = websocket.BinaryFrame
				isFinished = engine.EncodeTickFrame(buf, currentTPS)

				if err := websocket.Message.Send(ws, buf.Bytes()); err != nil {
					BufferPool.Put(buf)
					cancel()
					return
				}
			} else {
				// Mode de secours JSON textuel
				payload, ptSlice := engine.BuildFramePayloadWithPool(currentTPS)
				err := json.NewEncoder(buf).Encode(payload)
				simulation.PutPointSlice(ptSlice)

				if err != nil {
					BufferPool.Put(buf)
					cancel()
					return
				}

				if err := websocket.Message.Send(ws, buf.String()); err != nil {
					BufferPool.Put(buf)
					cancel()
					return
				}
				isFinished = payload.Stats.IsFinished
			}

			BufferPool.Put(buf)

			// Propager l'annulation précoce via context.WithCancel dès la fin de partie
			if isFinished {
				cancel()
				return
			}
		}
	}
}
