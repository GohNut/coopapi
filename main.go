package main

import (
    "log"
    "os"

    "github.com/joho/godotenv"

    "loan-dynamic-api/config"
    "loan-dynamic-api/routes"
)

func main() {
    // โหลด environment variables
    if err := godotenv.Load(); err != nil {
        log.Println("Warning: .env file not found")
    }

    // เริ่มต้น MongoDB Atlas connection
    if err := config.InitMongoAtlas(); err != nil {
        log.Fatalf("Failed to initialize MongoDB Atlas: %v", err)
    }
    defer config.DisconnectMongoAtlas()

    // Ensure Indexes
    if err := config.EnsureIndexes(); err != nil {
        log.Printf("Warning: Failed to ensure indexes: %v", err)
    }

    // สร้าง Echo instance
    e := routes.NewEcho()

    // เริ่มต้น server
    port := os.Getenv("API_PORT")
    if port == "" {
        port = "8080"
    }

    log.Printf("Starting Dynamic Loan API server on port %s", port)
    if err := e.Start(":" + port); err != nil {
        log.Fatal(err)
    }
}
