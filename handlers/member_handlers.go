package handlers

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/labstack/echo/v4"
	"go.mongodb.org/mongo-driver/bson"

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
			opts.Expires = 15 * 60 // 15 minutes
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

	// Build the full R2 URL (permanent URL)
	// Format: https://{bucket}.{accountid}.r2.cloudflarestorage.com/{key}
	// Note: For production, you might want to use a custom domain
	profileImageURL := fmt.Sprintf("https://%s/%s", bucket, r2Key)

	// Update the member's profile_image_url field
	filter := bson.M{"memberid": memberID}
	update := bson.M{
		"$set": bson.M{
			"profile_image_url": profileImageURL,
		},
	}

	result, err := collection.UpdateOne(context.TODO(), filter, update)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to update member record", "details": err.Error()})
	}

	if result.MatchedCount == 0 {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "Member not found"})
	}

	// Return both permanent URL and presigned URL
	response := map[string]interface{}{
		"status":            "success",
		"message":           "Profile image uploaded successfully",
		"profile_image_url": profileImageURL,
	}

	// Include presigned URL if available for immediate display
	if presignedURL != "" {
		response["presigned_url"] = presignedURL
	}

	return c.JSON(http.StatusOK, response)
}
