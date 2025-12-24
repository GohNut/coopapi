package handlers

import (
	"context"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"

	"loan-dynamic-api/config"
)

// NotificationGetRequest represents request to get notifications
type NotificationGetRequest struct {
	MemberID string `json:"memberid"`
}

// NotificationAddRequest represents request to add notification
type NotificationAddRequest struct {
	MemberID  string `json:"memberid"`
	Title     string `json:"title"`
	Message   string `json:"message"`
	Type      string `json:"type"`
	Route     string `json:"route,omitempty"`
}

// NotificationMarkReadRequest represents request to mark notification as read
type NotificationMarkReadRequest struct {
	MemberID       string `json:"memberid"`
	NotificationID string `json:"notification_id"`
}

// NotificationMarkAllReadRequest represents request to mark all notifications as read
type NotificationMarkAllReadRequest struct {
	MemberID string `json:"memberid"`
}

// NotificationDeleteRequest represents request to delete/clear notifications
type NotificationDeleteRequest struct {
	MemberID string `json:"memberid"`
}

// GetNotifications retrieves all notifications for a member
func GetNotifications(c echo.Context) error {
	var req NotificationGetRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"status":  "error",
			"message": "Invalid request",
			"error":   err.Error(),
		})
	}

	if req.MemberID == "" {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"status":  "error",
			"message": "memberid is required",
		})
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	db := config.GetDatabase()
	collection := db.Collection("notifications")

	// Find all notifications for this member, sorted by created_at descending
	filter := bson.M{"memberid": req.MemberID}
	opts := options.Find().SetSort(bson.D{{Key: "created_at", Value: -1}})

	cursor, err := collection.Find(ctx, filter, opts)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"status":  "error",
			"message": "Failed to retrieve notifications",
			"error":   err.Error(),
		})
	}
	defer cursor.Close(ctx)

	var notifications []bson.M
	if err = cursor.All(ctx, &notifications); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"status":  "error",
			"message": "Failed to decode notifications",
			"error":   err.Error(),
		})
	}

	if notifications == nil {
		notifications = []bson.M{}
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"status": "success",
		"count":  len(notifications),
		"data":   notifications,
	})
}

// AddNotification adds a new notification
func AddNotification(c echo.Context) error {
	var req NotificationAddRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"status":  "error",
			"message": "Invalid request",
			"error":   err.Error(),
		})
	}

	if req.MemberID == "" || req.Title == "" || req.Message == "" {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"status":  "error",
			"message": "memberid, title, and message are required",
		})
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	db := config.GetDatabase()
	collection := db.Collection("notifications")

	notification := bson.M{
		"memberid":   req.MemberID,
		"title":      req.Title,
		"message":    req.Message,
		"type":       req.Type,
		"is_read":    false,
		"created_at": time.Now(),
	}

	if req.Route != "" {
		notification["route"] = req.Route
	}

	result, err := collection.InsertOne(ctx, notification)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"status":  "error",
			"message": "Failed to add notification",
			"error":   err.Error(),
		})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"status":  "success",
		"message": "Notification added successfully",
		"id":      result.InsertedID,
	})
}

// MarkNotificationAsRead marks a single notification as read
func MarkNotificationAsRead(c echo.Context) error {
	var req NotificationMarkReadRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"status":  "error",
			"message": "Invalid request",
			"error":   err.Error(),
		})
	}

	if req.MemberID == "" || req.NotificationID == "" {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"status":  "error",
			"message": "memberid and notification_id are required",
		})
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	db := config.GetDatabase()
	collection := db.Collection("notifications")

	notificationObjID, err := primitive.ObjectIDFromHex(req.NotificationID)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"status":  "error",
			"message": "Invalid notification_id",
		})
	}

	filter := bson.M{
		"_id":      notificationObjID,
		"memberid": req.MemberID,
	}
	update := bson.M{
		"$set": bson.M{
			"is_read": true,
		},
	}

	result, err := collection.UpdateOne(ctx, filter, update)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"status":  "error",
			"message": "Failed to mark notification as read",
			"error":   err.Error(),
		})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"status":         "success",
		"message":        "Notification marked as read",
		"matched_count":  result.MatchedCount,
		"modified_count": result.ModifiedCount,
	})
}

// MarkAllNotificationsAsRead marks all notifications as read for a member
func MarkAllNotificationsAsRead(c echo.Context) error {
	var req NotificationMarkAllReadRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"status":  "error",
			"message": "Invalid request",
			"error":   err.Error(),
		})
	}

	if req.MemberID == "" {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"status":  "error",
			"message": "memberid is required",
		})
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	db := config.GetDatabase()
	collection := db.Collection("notifications")

	filter := bson.M{"memberid": req.MemberID}
	update := bson.M{
		"$set": bson.M{
			"is_read": true,
		},
	}

	result, err := collection.UpdateMany(ctx, filter, update)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"status":  "error",
			"message": "Failed to mark all notifications as read",
			"error":   err.Error(),
		})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"status":         "success",
		"message":        "All notifications marked as read",
		"modified_count": result.ModifiedCount,
	})
}

// ClearNotifications deletes all notifications for a member
func ClearNotifications(c echo.Context) error {
	var req NotificationDeleteRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"status":  "error",
			"message": "Invalid request",
			"error":   err.Error(),
		})
	}

	if req.MemberID == "" {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"status":  "error",
			"message": "memberid is required",
		})
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	db := config.GetDatabase()
	collection := db.Collection("notifications")

	filter := bson.M{"memberid": req.MemberID}

	result, err := collection.DeleteMany(ctx, filter)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"status":  "error",
			"message": "Failed to clear notifications",
			"error":   err.Error(),
		})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"status":        "success",
		"message":       "Notifications cleared",
		"deleted_count": result.DeletedCount,
	})
}
