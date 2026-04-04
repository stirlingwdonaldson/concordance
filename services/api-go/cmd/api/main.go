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
	"concordance/services/api-go/internal/db"
	"concordance/services/api-go/internal/documents"
	"concordance/services/api-go/internal/http/handlers"
	"concordance/services/api-go/internal/http/router"
	"concordance/services/api-go/internal/nlp"
	"concordance/services/api-go/internal/projects"
)

func main() {
	cfg := config.Load()
	dbPool, err := db.Connect(context.Background(), cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("database startup failed: %v", err)
	}
	defer dbPool.Close()

	if err := db.RunMigrations(context.Background(), dbPool, cfg.MigrationsDir); err != nil {
		log.Fatalf("migration startup failed: %v", err)
	}
	nlpSidecar, err := nlp.StartSidecar(cfg.NLPSidecarCmd, cfg.NLPSidecarScript, cfg.NLPGRPCPort())
	if err != nil {
		log.Fatalf("nlp sidecar startup failed: %v", err)
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
		defer cancel()
		if err := nlpSidecar.Stop(ctx); err != nil {
			log.Printf("nlp sidecar shutdown failed: %v", err)
		}
	}()

	nlpClient, err := nlp.NewGRPCClient(context.Background(), cfg.NLPGRPCAddr)
	if err != nil {
		log.Fatalf("nlp grpc connection failed: %v", err)
	}
	defer nlpClient.Close()

	readyCtx, cancelReady := context.WithTimeout(context.Background(), 10*time.Second)
	if err := nlp.WaitForReady(readyCtx, nlpClient); err != nil {
		cancelReady()
		log.Fatalf("nlp sidecar health check failed: %v", err)
	}
	cancelReady()

	health := handlers.NewHealthHandler(cfg, handlers.ReadinessChecks{
		Database: dbPool.Ping,
		NLPSidecar: func(ctx context.Context) error {
			_, err := nlpClient.Health(ctx)
			return err
		},
	})
	jobEvents := documents.NewJobEventBroker()
	projectStore := projects.NewPostgresStore(dbPool)
	projectHandlers := handlers.NewProjectsHandler(projectStore)
	documentStore := documents.NewPostgresStoreWithTikaEndpoint(dbPool, jobEvents, cfg.TikaEndpoint, nlpClient)
	documentHandlers := handlers.NewDocumentsHandler(documentStore, cfg.UploadDir, jobEvents)
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
