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

	// Unified API V1 routes
	v1 := e.Group("/api/v1")

	// Dynamic CRUD operations (Previously under /loan)
	v1.POST("/create", handlers.LoanDynamicCreate)
	v1.POST("/get", handlers.LoanDynamicGet)
	v1.POST("/update", handlers.LoanDynamicUpdate)
	v1.POST("/delete", handlers.LoanDynamicDelete)
	v1.POST("/verify-token", handlers.VerifyTokenHandler)

	// Document endpoints
	v1.POST("/document/upload", handlers.DocumentUploadHandler)
	v1.POST("/document/list", handlers.DocumentListHandler)
	v1.POST("/document/get", handlers.DocumentGetHandler)
	v1.POST("/document/info", handlers.DocumentInfoHandler)
	v1.POST("/document/delete", handlers.DocumentDeleteHandler)

	// Member endpoints
	v1.POST("/upload-profile-image", handlers.UploadProfileImageHandler)
	v1.GET("/member/profile-image", handlers.GetMemberProfileImageHandler)
	v1.GET("/member/profile-image/proxy", handlers.ProxyProfileImageHandler)
	
	// KYC
	v1.POST("/member/kyc", handlers.SubmitKYC)
	
	// Officer KYC (Should be protected by Officer Middleware in real implementation)
	v1.GET("/officer/kyc/pending", handlers.GetPendingKYC)
	v1.GET("/officer/kyc/detail/:memberID", handlers.GetKYCDetail)
	v1.POST("/officer/kyc/review", handlers.ReviewKYC)

	// Share Management
	v1.POST("/share/create", handlers.CreateShareType)
	v1.POST("/share/update/:id", handlers.UpdateShareType)
	v1.GET("/share/list", handlers.GetShareTypes)
	v1.DELETE("/share/delete/:id", handlers.DeleteShareType)
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
