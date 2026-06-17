package main

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/joho/godotenv"
	"rentspace/backend/internal/api"
	"rentspace/backend/internal/config"
	"rentspace/backend/internal/store"
)

func runBookingWorker(ctx context.Context, q *store.SQLStore) {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			completed, err := q.BulkCompleteConfirmedBookings(ctx)
			if err != nil {
				log.Printf("booking worker: complete: %v", err)
			} else if len(completed) > 0 {
				log.Printf("booking worker: auto-completed %d bookings", len(completed))
			}

			expired, err := q.BulkExpirePendingBookings(ctx)
			if err != nil {
				log.Printf("booking worker: expire: %v", err)
			} else if len(expired) > 0 {
				log.Printf("booking worker: auto-cancelled %d expired pending bookings", len(expired))
			}
		}
	}
}

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

	queries := store.NewStore(db)
	router := api.NewRouter(queries, cfg.JWTSecret, cfg.CORSOrigins, cfg.GotenbergURL)

	// Background worker: auto-complete confirmed bookings past end_time,
	// auto-cancel expired pending bookings (expires_at <= NOW()).
	go runBookingWorker(ctx, queries)

	server := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      router,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	log.Printf("server listening on :%s", cfg.Port)
	if err := server.ListenAndServe(); err != nil {
		log.Fatalf("server: %v", err)
	}
}
