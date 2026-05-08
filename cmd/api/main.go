// @title           RentSpace API
// @version         1.0
// @description     Short-term space rental marketplace API for Thailand
// @host            localhost:8080
// @BasePath        /api/v1

package main

import (
	"context"
	"log"
	"net/http"

	"github.com/joho/godotenv"
	"rentspace/backend/internal/api"
	"rentspace/backend/internal/config"
	"rentspace/backend/internal/store"
	_ "rentspace/backend/docs"
)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Println("no .env file found, using environment variables")
	}

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	ctx := context.Background()

	db, err := store.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("database: %v", err)
	}
	defer db.Close()

	queries := store.New(db)
	router := api.NewRouter(queries, cfg.JWTSecret)

	log.Printf("server listening on :%s", cfg.Port)
	if err := http.ListenAndServe(":"+cfg.Port, router); err != nil {
		log.Fatalf("server: %v", err)
	}
}
