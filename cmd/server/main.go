package main

import (
	"context"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/IliaPopov28/websocket-chat/internal/auth"
	"github.com/IliaPopov28/websocket-chat/internal/hub"
	"github.com/IliaPopov28/websocket-chat/internal/store/postgres"
	"github.com/IliaPopov28/websocket-chat/internal/transport"
	"github.com/IliaPopov28/websocket-chat/web"
)

// webFiles возвращает fs.FS с содержимым web/.
func webFiles() fs.FS {
	sub, err := fs.Sub(web.WebFS, ".")
	if err != nil {
		panic(err)
	}
	return sub
}

func main() {
	// 1. Подключение к PostgreSQL с повторными попытками.
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = "postgres://wschat:wschat_secret@localhost:5432/wschat"
	}

	var pool *pgxpool.Pool
	var err error
	for i := 0; i < 10; i++ {
		pool, err = pgxpool.New(context.Background(), dsn)
		if err != nil {
			log.Printf("Failed to create pool (attempt %d/10): %v", i+1, err)
			time.Sleep(3 * time.Second)
			continue
		}

		if err = pool.Ping(context.Background()); err == nil {
			break
		}

		log.Printf("Failed to ping database (attempt %d/10): %v", i+1, err)
		pool.Close()
		time.Sleep(3 * time.Second)
	}
	if err != nil {
		log.Fatalf("Unable to connect to database after retries: %v\n", err)
	}
	defer pool.Close()

	log.Println("Connected to PostgreSQL")

	// 2. Инициализация сервисов.
	userStore := postgres.NewUserStore(pool)

	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		jwtSecret = "super-secret-key-change-in-production"
	}

	authService := auth.NewService(userStore, jwtSecret)

	// 3. Hub.
	h := hub.NewHub()
	go h.Run()

	// 4. Handler.
	handler := transport.NewHandler(h, authService)

	// 5. Маршруты.
	mux := http.NewServeMux()

	mux.HandleFunc("/ws", handler.HandleWebSocket)
	mux.HandleFunc("/api/register", handler.HandleRegister)
	mux.HandleFunc("/api/login", handler.HandleLogin)

	mux.Handle("/", http.FileServer(http.FS(webFiles())))

	// 6. HTTP сервер.
	server := &http.Server{
		Addr:    ":8081",
		Handler: mux,
	}

	// 7. Graceful shutdown.
	go func() {
		sigCh := make(chan os.Signal, 2)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh
		log.Println("Shutting down server...")

		// 1. Закрываем все WebSocket-соединения — это разорвёт long-lived HTTP connections.
		h.Shutdown()

		// 2. Теперь server.Shutdown() сможет завершиться, т.к. все WS-соединения закрыты.
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := server.Shutdown(ctx); err != nil {
			log.Printf("Server forced to shutdown: %v", err)
		}

		pool.Close()
	}()

	log.Println("Server starting on :8081")
	if err := server.ListenAndServe(); err != http.ErrServerClosed {
		log.Fatalf("Server error: %v", err)
	}
	log.Println("Server exited properly")
}
