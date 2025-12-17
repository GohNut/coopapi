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
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"

	"loan-dynamic-api/config"
)

// DocumentMetadata represents the metadata stored in MongoDB
type DocumentMetadata struct {
	ID          primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	RefID       string             `bson:"ref_id" json:"ref_id"` // Renamed from ShopID
	Filename    string             `bson:"filename" json:"filename"`
	Category    string             `bson:"category,omitempty" json:"category,omitempty"`
	Description string             `bson:"description,omitempty" json:"description,omitempty"`
	Tags        []string           `bson:"tags,omitempty" json:"tags,omitempty"`
	UploadedBy  string             `bson:"uploaded_by,omitempty" json:"uploaded_by,omitempty"`
	R2Key       string             `bson:"r2_key" json:"r2_key,omitempty"`
	ContentType string             `bson:"content_type" json:"content_type"`
	Size        int64              `bson:"size" json:"size"`
	UploadDate  time.Time          `bson:"upload_date" json:"upload_date"`
}

// Request structs
type DocumentListRequest struct {
	RefID       string `json:"ref_id"`
	Category    string `json:"category,omitempty"`
	Limit       int64  `json:"limit,omitempty"`
	Skip        int64  `json:"skip,omitempty"`
	IncludeData bool   `json:"include_data,omitempty"`
}

type DocumentGetRequest struct {
	RefID    string `json:"ref_id"`
	Filename string `json:"filename"`
}

type DocumentDeleteRequest struct {
	RefID    string `json:"ref_id"`
	DocID    string `json:"doc_id,omitempty"`
	Filename string `json:"filename,omitempty"`
}

const MaxFileSize = 10 * 1024 * 1024 // 10MB

// DocumentUploadHandler handles document upload (Image/PDF)
func DocumentUploadHandler(c echo.Context) error {
	// 1. Parse Multipart Form
	file, err := c.FormFile("file")
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "File is required", "details": err.Error()})
	}

	// Validate Size
	if file.Size > MaxFileSize {
		return c.JSON(http.StatusRequestEntityTooLarge, map[string]string{"error": "File size exceeds limit (10MB)"})
	}

	src, err := file.Open()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to open file"})
	}
	defer src.Close()

	refID := c.FormValue("ref_id")
	if refID == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "ref_id is required"})
	}

	category := c.FormValue("category")
	description := c.FormValue("description")
	uploadedBy := c.FormValue("uploaded_by")
	tags := c.FormValue("tags")

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

	// Generate unique R2 key
	ext := strings.ToLower(filepath.Ext(file.Filename))
	uniqueID := uuid.New().String()
	r2Key := fmt.Sprintf("%s/%s%s", refID, uniqueID, ext)

	contentType := file.Header.Get("Content-Type")
	// Fallback/Ensure PDF/Image types
	if contentType == "" || contentType == "application/octet-stream" {
		if ext == ".pdf" {
			contentType = "application/pdf"
		} else if ext == ".png" {
			contentType = "image/png"
		} else if ext == ".jpg" || ext == ".jpeg" {
			contentType = "image/jpeg"
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

	// 3. Save to MongoDB
	db := config.GetDatabase()
	if db == nil {
		return c.JSON(http.StatusServiceUnavailable, map[string]string{"error": "Database not connected"})
	}
	collection := db.Collection("documents")

	metadata := DocumentMetadata{
		ID:          primitive.NewObjectID(),
		RefID:       refID,
		Filename:    uniqueID + ext,
		Category:    category,
		Description: description,
		UploadedBy:  uploadedBy,
		R2Key:       r2Key,
		ContentType: contentType,
		Size:        file.Size,
		UploadDate:  time.Now(),
	}
	if tags != "" {
		metadata.Tags = strings.Split(tags, ",")
	}

	_, err = collection.InsertOne(context.TODO(), metadata)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to save metadata", "details": err.Error()})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"status":   "success",
		"message":  "Document uploaded successfully",
		"filename": metadata.Filename,
		"doc_id":   metadata.ID.Hex(),
	})
}

// DocumentListHandler lists documents
func DocumentListHandler(c echo.Context) error {
	var req DocumentListRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid request"})
	}

	if req.RefID == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "ref_id is required"})
	}

	db := config.GetDatabase()
	if db == nil {
		return c.JSON(http.StatusServiceUnavailable, map[string]string{"error": "Database not connected"})
	}
	collection := db.Collection("documents")

	filter := bson.M{"ref_id": req.RefID}
	if req.Category != "" {
		filter["category"] = req.Category
	}

	opts := options.Find().SetSort(bson.M{"upload_date": -1})
	if req.Limit > 0 {
		opts.SetLimit(req.Limit)
	}
	if req.Skip > 0 {
		opts.SetSkip(req.Skip)
	}

	cursor, err := collection.Find(context.TODO(), filter, opts)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to query database"})
	}
	defer cursor.Close(context.TODO())

	var documents []DocumentMetadata
	if err := cursor.All(context.TODO(), &documents); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to decode results"})
	}

	var response []map[string]interface{}
	bucket := config.GetR2Bucket()

	for _, doc := range documents {
		item := map[string]interface{}{
			"id":           doc.ID.Hex(),
			"ref_id":       doc.RefID,
			"filename":     doc.Filename,
			"category":     doc.Category,
			"description":  doc.Description,
			"upload_date":  doc.UploadDate,
			"content_type": doc.ContentType,
			"size":         doc.Size,
		}

		if req.IncludeData {
			presignClient := config.GetR2PresignClient()
			if presignClient != nil {
				presignedRequest, err := presignClient.PresignGetObject(context.TODO(), &s3.GetObjectInput{
					Bucket: aws.String(bucket),
					Key:    aws.String(doc.R2Key),
				}, func(opts *s3.PresignOptions) {
					opts.Expires = 15 * time.Minute
				})

				if err == nil {
					item["url"] = presignedRequest.URL
				}
			}
		}
		response = append(response, item)
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"status": "success",
		"data":   response,
	})
}

// DocumentGetHandler retrieves a single document URL
func DocumentGetHandler(c echo.Context) error {
	var req DocumentGetRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid request"})
	}

	if req.RefID == "" || req.Filename == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "ref_id and filename are required"})
	}

	db := config.GetDatabase()
	if db == nil {
		return c.JSON(http.StatusServiceUnavailable, map[string]string{"error": "Database not connected"})
	}
	collection := db.Collection("documents")

	var metadata DocumentMetadata
	err := collection.FindOne(context.TODO(), bson.M{"ref_id": req.RefID, "filename": req.Filename}).Decode(&metadata)
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "Document not found"})
	}

	presignClient := config.GetR2PresignClient()
	if presignClient == nil {
		return c.JSON(http.StatusServiceUnavailable, map[string]string{"error": "R2 storage not configured"})
	}
	bucket := config.GetR2Bucket()

	presignedRequest, err := presignClient.PresignGetObject(context.TODO(), &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(metadata.R2Key),
	}, func(opts *s3.PresignOptions) {
		opts.Expires = 15 * time.Minute
	})

	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to generate presigned URL", "details": err.Error()})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"status":       "success",
		"filename":     metadata.Filename,
		"content_type": metadata.ContentType,
		"url":          presignedRequest.URL,
		"expires_in":   "15m",
	})
}

// DocumentInfoHandler retrieves document metadata
func DocumentInfoHandler(c echo.Context) error {
	var req DocumentGetRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid request"})
	}

	if req.RefID == "" || req.Filename == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "ref_id and filename are required"})
	}

	db := config.GetDatabase()
	if db == nil {
		return c.JSON(http.StatusServiceUnavailable, map[string]string{"error": "Database not connected"})
	}
	collection := db.Collection("documents")

	var metadata DocumentMetadata
	err := collection.FindOne(context.TODO(), bson.M{"ref_id": req.RefID, "filename": req.Filename}).Decode(&metadata)
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "Document not found"})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"status": "success",
		"data":   metadata,
	})
}

// DocumentDeleteHandler deletes a document
func DocumentDeleteHandler(c echo.Context) error {
	var req DocumentDeleteRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid request"})
	}

	if req.RefID == "" || (req.DocID == "" && req.Filename == "") {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "ref_id and either doc_id or filename are required"})
	}

	db := config.GetDatabase()
	if db == nil {
		return c.JSON(http.StatusServiceUnavailable, map[string]string{"error": "Database not connected"})
	}
	collection := db.Collection("documents")

	filter := bson.M{"ref_id": req.RefID}
	if req.DocID != "" {
		oid, err := primitive.ObjectIDFromHex(req.DocID)
		if err == nil {
			filter["_id"] = oid
		} else {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid doc_id"})
		}
	} else {
		filter["filename"] = req.Filename
	}

	var metadata DocumentMetadata
	err := collection.FindOne(context.TODO(), filter).Decode(&metadata)
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "Document not found"})
	}

	// Delete from R2
	r2Client := config.GetR2Client()
	if r2Client != nil {
		bucket := config.GetR2Bucket()
		_, err := r2Client.DeleteObject(context.TODO(), &s3.DeleteObjectInput{
			Bucket: aws.String(bucket),
			Key:    aws.String(metadata.R2Key),
		})
		if err != nil {
			fmt.Printf("Warning: Failed to delete from R2: %v\n", err)
		}
	}

	_, err = collection.DeleteOne(context.TODO(), filter)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to delete from database"})
	}

	return c.JSON(http.StatusOK, map[string]string{"status": "success", "message": "Document deleted"})
}
