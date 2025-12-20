package routes

import (
	"loan-dynamic-api/handlers"
	"os"
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

func NewEcho() *echo.Echo {
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

	return e
}

func setupRoutes(e *echo.Echo) {
	// Health check
	e.GET("/health", func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]string{
			"status":  "ok",
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

	// Document endpoints (Refactored from Image API)
	e.POST("/document/upload", handlers.DocumentUploadHandler)
	e.POST("/document/list", handlers.DocumentListHandler)
	e.POST("/document/get", handlers.DocumentGetHandler)
	e.POST("/document/info", handlers.DocumentInfoHandler)
	e.POST("/document/delete", handlers.DocumentDeleteHandler)

	// Member endpoints
	e.POST("/upload-profile-image", handlers.UploadProfileImageHandler)
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
