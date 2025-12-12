package main

import (
    "log"
    "os"

    "github.com/joho/godotenv"
    "github.com/labstack/echo/v4"
    "github.com/labstack/echo/v4/middleware"

    "loan-dynamic-api/config"
    "loan-dynamic-api/handlers"
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
    e := echo.New()

    // Middleware
    e.Use(middleware.Logger())
    e.Use(middleware.Recover())
    e.Use(middleware.CORSWithConfig(middleware.CORSConfig{
        AllowOrigins: getAllowedOrigins(),
        AllowMethods: []string{echo.GET, echo.POST, echo.PUT, echo.DELETE, echo.OPTIONS},
        AllowHeaders: []string{echo.HeaderContentType, echo.HeaderAuthorization},
    }))

    // Routes
    setupRoutes(e)

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

func setupRoutes(e *echo.Echo) {
    // Health check
    e.GET("/health", func(c echo.Context) error {
        return c.JSON(200, map[string]string{
            "status": "ok",
            "service": "loan-dynamic-api",
        })
    })

    // Dynamic Loan API routes
    loanGroup := e.Group("/api/v1/loan")
    
    // Dynamic CRUD operations
    loanGroup.POST("/create", handlers.LoanDynamicCreate)
    loanGroup.POST("/get", handlers.LoanDynamicGet)
    loanGroup.POST("/update", handlers.LoanDynamicUpdate)
    loanGroup.POST("/delete", handlers.LoanDynamicDelete)
}

func getAllowedOrigins() []string {
    corsOrigins := os.Getenv("CORS_ORIGINS")
    if corsOrigins == "" {
        return []string{"*"}
    }
    // Simple split if multiple origins are supported, but for now just returning the value as single or split if needed
    // Ideally this should split by comma if multiple origins are provided
    return []string{corsOrigins}
}
