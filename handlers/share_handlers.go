package handlers

import (
	"context"
	"net/http"
	"time"

	"loan-dynamic-api/config"
	"loan-dynamic-api/models"

	"github.com/labstack/echo/v4"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// CreateShareType - Create a new share type
func CreateShareType(c echo.Context) error {
	var shareType models.ShareType
	if err := c.Bind(&shareType); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"status":  "error",
			"message": "Invalid request body",
		})
	}

	shareType.ID = primitive.NewObjectID()
	shareType.Status = "active" // Default to active
	shareType.CreatedAt = time.Now()
	shareType.UpdatedAt = time.Now()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	collection := config.GetDatabase().Collection("share_types")
	_, err := collection.InsertOne(ctx, shareType)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"status":  "error",
			"message": "Failed to create share type",
			"error":   err.Error(),
		})
	}

	return c.JSON(http.StatusCreated, map[string]interface{}{
		"status": "success",
		"data":   shareType,
	})
}

// UpdateShareType - Update an existing share type
func UpdateShareType(c echo.Context) error {
	id := c.Param("id")
	objectID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"status":  "error",
			"message": "Invalid ID format",
		})
	}

	var updateData map[string]interface{}
	if err := c.Bind(&updateData); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"status":  "error",
			"message": "Invalid request body",
		})
	}

	updateData["updated_at"] = time.Now()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	collection := config.GetDatabase().Collection("share_types")
	filter := bson.M{"_id": objectID}
	update := bson.M{"$set": updateData}

	_, err = collection.UpdateOne(ctx, filter, update)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"status":  "error",
			"message": "Failed to update share type",
			"error":   err.Error(),
		})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"status": "success",
		"message": "Share type updated successfully",
	})
}

// GetShareTypes - Get all share types
func GetShareTypes(c echo.Context) error {
    // Optional: Filter by status if query param provided
    status := c.QueryParam("status")
    
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

    filter := bson.M{}
    if status != "" {
        filter["status"] = status
    } else {
        // Default might be returning everything, or just active? 
        // Let's return all for officer management, filter in frontend if needed.
        // Or if strict:
        // filter = bson.M{}
    }

	collection := config.GetDatabase().Collection("share_types")
    
    // Sort by name or created_at
    opts := options.Find().SetSort(bson.D{{"name", 1}})
    
	cursor, err := collection.Find(ctx, filter, opts)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"status":  "error",
			"message": "Failed to fetch share types",
			"error":   err.Error(),
		})
	}
	defer cursor.Close(ctx)

	var shareTypes []models.ShareType
	if err := cursor.All(ctx, &shareTypes); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"status":  "error",
			"message": "Failed to decode share types",
			"error":   err.Error(),
		})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"status": "success",
		"data":   shareTypes,
	})
}

// DeleteShareType - Soft delete share type
func DeleteShareType(c echo.Context) error {
	id := c.Param("id")
	objectID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"status":  "error",
			"message": "Invalid ID format",
		})
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	collection := config.GetDatabase().Collection("share_types")
	
    // Soft delete by setting status to inactive
    update := bson.M{"$set": bson.M{"status": "inactive", "updated_at": time.Now()}}
	_, err = collection.UpdateOne(ctx, bson.M{"_id": objectID}, update)
	
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"status":  "error",
			"message": "Failed to delete share type",
			"error":   err.Error(),
		})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"status": "success",
		"message": "Share type deleted successfully (soft delete)",
	})
}
