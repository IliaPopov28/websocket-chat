package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"syscall"
	"time"

	"github.com/IliaPopov28/websocket-chat/internal/hub"
	"github.com/IliaPopov28/websocket-chat/internal/transport"
)

// webDir определяет путь к директории web/ относительно расположения main.go.
// runtime.Caller(0) возвращает путь к этому файлу (main.go).
// От него поднимаемся на 2 уровня вверх: cmd/server/ -> корень проекта.
func webDir() string {
	_, filename, _, _ := runtime.Caller(0)
	projectRoot := filepath.Dir(filepath.Dir(filepath.Dir(filename)))
	return filepath.Join(projectRoot, "web")
}

func main() {
	h := hub.NewHub()
	go h.Run()

	handler := transport.NewHandler(h)

	mux := http.NewServeMux()

	mux.HandleFunc("/ws", handler.HandleWebSocket)
	mux.HandleFunc("/api/check-nick", handler.HandleCheckNick)

	mux.Handle("/", http.FileServer(http.Dir(webDir())))

	server := &http.Server{
		Addr:    ":8081",
		Handler: mux,
	}

	go func() {
		sigCh := make(chan os.Signal, 2)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh
		log.Println("Shutting down server...")

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := server.Shutdown(ctx); err != nil {
			log.Printf("Server forced to shutdown: %v", err)
		}
	}()

	log.Println("Server starting on :8081")
	if err := server.ListenAndServe(); err != http.ErrServerClosed {
		log.Fatalf("Server error: %v", err)
	}
	log.Println("Server exited properly")
}
