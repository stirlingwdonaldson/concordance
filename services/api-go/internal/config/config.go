package config

import (
	"fmt"
	"os"
)

type Config struct {
	Port            string
	PipelineVersion string
	EmbeddingModel  string
	EmbeddingDim    int
	UploadDir       string
	DatabaseURL     string
	MigrationsDir   string
}

func Load() Config {
	port := os.Getenv("API_PORT")
	if port == "" {
		port = "8080"
	}

	return Config{
		Port:            port,
		PipelineVersion: envOrDefault("PIPELINE_VERSION", "v0"),
		EmbeddingModel:  envOrDefault("EMBEDDING_MODEL", "all-MiniLM-L6-v2"),
		EmbeddingDim:    384,
		UploadDir:       envOrDefault("UPLOAD_DIR", ".concordance-data/uploads"),
		DatabaseURL:     os.Getenv("DATABASE_URL"),
		MigrationsDir:   envOrDefault("MIGRATIONS_DIR", "migrations"),
	}
}

func (c Config) Addr() string {
	return fmt.Sprintf(":%s", c.Port)
}

func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}

	return fallback
}
