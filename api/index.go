package handler

import (
	"log"
	"net/http"

	"github.com/joho/godotenv"
	"github.com/labstack/echo/v4"
	"loan-dynamic-api/config"
	"loan-dynamic-api/routes"
)

var e *echo.Echo

// Handler is the entry point for Vercel Serverless Function
func Handler(w http.ResponseWriter, r *http.Request) {
	if e == nil {
		// Load .env if present (Vercel uses env vars from dashboard)
		_ = godotenv.Load()

		// Init DB
		// Note: InitMongoAtlas handles connection reuse logic internally now
		if err := config.InitMongoAtlas(); err != nil {
			log.Printf("Failed to init DB: %v", err)
			http.Error(w, "Database connection failed", http.StatusInternalServerError)
			return
		}

		// Ensure Indexes
		if err := config.EnsureIndexes(); err != nil {
			log.Printf("Warning: Failed to ensure indexes: %v", err)
		}

		// Create Echo instance
		e = routes.NewEcho()
	}

	e.ServeHTTP(w, r)
}
