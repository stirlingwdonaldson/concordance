package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"concordance/services/api-go/internal/config"
	"concordance/services/api-go/internal/documents"
	"concordance/services/api-go/internal/http/handlers"
	"concordance/services/api-go/internal/http/router"
	"concordance/services/api-go/internal/projects"
)

func main() {
	cfg := config.Load()
	health := handlers.NewHealthHandler(cfg)
	projectStore := projects.NewMemoryStore()
	projectHandlers := handlers.NewProjectsHandler(projectStore)
	documentStore := documents.NewMemoryStore()
	documentHandlers := handlers.NewDocumentsHandler(documentStore, cfg.UploadDir)
	h := router.New(health, projectHandlers, documentHandlers)

	server := http.Server{
		Addr:              cfg.Addr(),
		Handler:           h,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		log.Printf("api-go listening on %s", cfg.Addr())
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("server failed: %v", err)
		}
	}()

	waitForShutdown(&server)
}

func waitForShutdown(server *http.Server) {
	signalCtx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	<-signalCtx.Done()
	log.Println("shutdown signal received")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Printf("shutdown failed: %v", err)
		os.Exit(1)
	}

	log.Println("server stopped")
}
