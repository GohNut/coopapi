package handlers

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/labstack/echo/v4"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"

	"loan-dynamic-api/config"
)

const MaxProfileImageSize = 5 * 1024 * 1024 // 5MB

// UploadProfileImageHandler handles profile image upload for members
func UploadProfileImageHandler(c echo.Context) error {
	// 1. Parse Multipart Form
	file, err := c.FormFile("file")
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "File is required", "details": err.Error()})
	}

	// Validate Size
	if file.Size > MaxProfileImageSize {
		return c.JSON(http.StatusRequestEntityTooLarge, map[string]string{"error": "File size exceeds limit (5MB)"})
	}

	memberID := c.FormValue("memberid")
	if memberID == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "memberid is required"})
	}

	// Validate file type
	ext := strings.ToLower(filepath.Ext(file.Filename))
	allowedExts := map[string]bool{
		".jpg":  true,
		".jpeg": true,
		".png":  true,
		".webp": true,
	}
	if !allowedExts[ext] {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Only JPG, PNG, and WebP images are allowed"})
	}

	src, err := file.Open()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to open file"})
	}
	defer src.Close()

	// Read file content
	buffer := bytes.NewBuffer(nil)
	if _, err := io.Copy(buffer, src); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to read file"})
	}
	fileBytes := buffer.Bytes()

	// 2. Upload to R2
	r2Client := config.GetR2Client()
	if r2Client == nil {
		return c.JSON(http.StatusServiceUnavailable, map[string]string{"error": "R2 storage not configured"})
	}
	bucket := config.GetR2Bucket()

	// Generate R2 key for profile image
	r2Key := fmt.Sprintf("members/%s/profile%s", memberID, ext)

	contentType := file.Header.Get("Content-Type")
	// Fallback/Ensure image types
	if contentType == "" || contentType == "application/octet-stream" {
		if ext == ".png" {
			contentType = "image/png"
		} else if ext == ".jpg" || ext == ".jpeg" {
			contentType = "image/jpeg"
		} else if ext == ".webp" {
			contentType = "image/webp"
		}
	}

	_, err = r2Client.PutObject(context.TODO(), &s3.PutObjectInput{
		Bucket:      aws.String(bucket),
		Key:         aws.String(r2Key),
		Body:        bytes.NewReader(fileBytes),
		ContentType: aws.String(contentType),
	})
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to upload to storage", "details": err.Error()})
	}

	// 3. Generate presigned URL for immediate use
	presignClient := config.GetR2PresignClient()
	var presignedURL string
	if presignClient != nil {
		presignedRequest, err := presignClient.PresignGetObject(context.TODO(), &s3.GetObjectInput{
			Bucket: aws.String(bucket),
			Key:    aws.String(r2Key),
		}, func(opts *s3.PresignOptions) {
			opts.Expires = 24 * time.Hour // 24 hours for longer validity
		})
		if err == nil {
			presignedURL = presignedRequest.URL
		}
	}

	// 4. Update member record in MongoDB
	db := config.GetDatabase()
	if db == nil {
		return c.JSON(http.StatusServiceUnavailable, map[string]string{"error": "Database not connected"})
	}
	collection := db.Collection("members")

	// Store the R2 key in database (not the full URL)
	// This allows us to generate presigned URLs on-demand
	filter := bson.M{"memberid": memberID}
	update := bson.M{
		"$set": bson.M{
			"profile_image_key":        r2Key,
			"profile_image_updated_at": time.Now(),
		},
	}

	result, err := collection.UpdateOne(context.TODO(), filter, update)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to update member record", "details": err.Error()})
	}

	if result.MatchedCount == 0 {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "Member not found"})
	}

	// Return presigned URL for immediate use with version for cache busting
	response := map[string]interface{}{
		"status":            "success",
		"message":           "Profile image uploaded successfully",
		"profile_image_url": presignedURL, // Return presigned URL
		"version":           time.Now().Unix(), // Add version for cache busting
	}

	return c.JSON(http.StatusOK, response)
}

// GetMemberProfileImageHandler generates a fresh presigned URL for member's profile image
func GetMemberProfileImageHandler(c echo.Context) error {
	memberID := c.QueryParam("memberid")
	if memberID == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "memberid is required"})
	}

	// Get member from database
	db := config.GetDatabase()
	if db == nil {
		return c.JSON(http.StatusServiceUnavailable, map[string]string{"error": "Database not connected"})
	}
	collection := db.Collection("members")

	var member map[string]interface{}
	err := collection.FindOne(context.TODO(), bson.M{"memberid": memberID}).Decode(&member)
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "Member not found"})
	}

	// Check if member has profile image
	profileImageKey, ok := member["profile_image_key"].(string)
	if !ok || profileImageKey == "" {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "No profile image found"})
	}

	// Generate presigned URL
	presignClient := config.GetR2PresignClient()
	if presignClient == nil {
		return c.JSON(http.StatusServiceUnavailable, map[string]string{"error": "R2 storage not configured"})
	}
	bucket := config.GetR2Bucket()

	presignedRequest, err := presignClient.PresignGetObject(context.TODO(), &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(profileImageKey),
	}, func(opts *s3.PresignOptions) {
		opts.Expires = 24 * time.Hour // 24 hours
	})

	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to generate presigned URL", "details": err.Error()})
	}

	// Get version from updated_at timestamp
	var version int64
	if updatedAt, ok := member["profile_image_updated_at"].(primitive.DateTime); ok {
		version = updatedAt.Time().Unix()
	} else {
		// Fallback to current time if no timestamp exists
		version = time.Now().Unix()
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"status":            "success",
		"profile_image_url": presignedRequest.URL,
		"version":           version,
		"expires_in":        "24h",
	})
}

// ProxyProfileImageHandler serves profile image through backend to avoid CORS issues
func ProxyProfileImageHandler(c echo.Context) error {
	memberID := c.QueryParam("memberid")
	if memberID == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "memberid is required"})
	}

	// Get member from database
	db := config.GetDatabase()
	if db == nil {
		return c.JSON(http.StatusServiceUnavailable, map[string]string{"error": "Database not connected"})
	}
	collection := db.Collection("members")

	var member map[string]interface{}
	err := collection.FindOne(context.TODO(), bson.M{"memberid": memberID}).Decode(&member)
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "Member not found"})
	}

	// Check if member has profile image
	profileImageKey, ok := member["profile_image_key"].(string)
	if !ok || profileImageKey == "" {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "No profile image found"})
	}

	// Get image from R2
	r2Client := config.GetR2Client()
	if r2Client == nil {
		return c.JSON(http.StatusServiceUnavailable, map[string]string{"error": "R2 storage not configured"})
	}
	bucket := config.GetR2Bucket()

	result, err := r2Client.GetObject(context.TODO(), &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(profileImageKey),
	})
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "Image not found in storage"})
	}
	defer result.Body.Close()

	// Set appropriate headers
	contentType := "image/jpeg" // default
	if result.ContentType != nil {
		contentType = *result.ContentType
	}

	// Cache for 24 hours
	c.Response().Header().Set("Content-Type", contentType)
	c.Response().Header().Set("Cache-Control", "public, max-age=86400") // 24 hours
	c.Response().Header().Set("Access-Control-Allow-Origin", "*") // Allow CORS

	// Stream the image
	return c.Stream(http.StatusOK, contentType, result.Body)
}
