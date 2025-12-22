package handlers

import (
	"context"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"go.mongodb.org/mongo-driver/bson"
	"loan-dynamic-api/config"
)

// VerifyTokenHandler verifies an iLife token and returns member info
func VerifyTokenHandler(c echo.Context) error {
	var req struct {
		Token string `json:"token"`
	}
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid request body"})
	}

	if req.Token == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Token is required"})
	}

	// In a real implementation, we would verify this token with iLife API
	// or check our local session/token store.
	// For now, let's assume the token can be the memberID for testing, 
	// or we look up a member who has this token.
	
	db := config.GetDatabase()
	if db == nil {
		return c.JSON(http.StatusServiceUnavailable, map[string]string{"error": "Database not connected"})
	}
	collection := db.Collection("members")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Try to find member by memberid (using token as id for demo) 
	// OR find by a specific token field if we add it.
	filter := bson.M{
		"$or": []bson.M{
			{"memberid": req.Token},
			{"sso_token": req.Token},
		},
	}

	var member map[string]interface{}
	err := collection.FindOne(ctx, filter).Decode(&member)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, map[string]interface{}{
			"status": "error",
			"message": "Invalid or expired token",
		})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"status": "success",
		"data":   member,
	})
}
