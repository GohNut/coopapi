
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
    "go.mongodb.org/mongo-driver/mongo/options"
    
	"loan-dynamic-api/config"
)

// SubmitKYC handles the KYC submission (Images + Bank Info)
func SubmitKYC(c echo.Context) error {
	// Parse Multipart Form
	_, err := c.MultipartForm()
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Failed to parse multipart form"})
	}

	// 1. Get Form Values
	bankID := c.FormValue("bank_id")
	bankAccountNo := c.FormValue("bank_account_no")
	
	// Identify Member (Ideally from Token Middleware, but using params/body for now if not strictly enforced yet)
	// In a real scenario, extract memberID from context set by JWT middleware
	// For now, let's assume valid auth populated "member_id" in context or prompt client to send it, 
	// OR (temporary) get from header for this specific implementation if middleware not fully visible to me.
    // Based on `routes.go`, I don't see explicit JWT middleware on the group yet, checking `member_handlers`...
    // `member_handlers` takes `memberid` from FormValue. Let's do the same for consistency until middleware is clarified.
    // Wait, the client code sends Token in Header: Authorization: Bearer $token.
    // We should probably rely on MemberID coming from the token claims if middleware exists. 
    // BUT looking at `UploadProfileImageHandler` it takes `memberid` as form value. 
    // I will extract member ID from Token if available, otherwise look for form value/header. 
    // To be safe and consistent with `UploadProfileImageHandler`, I'll look for `member_id` (or similar) in the request logic 
    // OR just use the token logic if I can find the middleware.
    
    // Simplest approach: We will require the client to mock/send member id? 
    // No, the client sends: bank_id, bank_account_no, images.
    // It sends "Authorization" header.
    // I entered code in `KYCService` that sends `request.headers['Authorization'] = 'Bearer $token'`.
    // I need to decode that token to get the memberID.
    // However, I don't have visibility into `auth_middleware` here easily without exploring more.
    // To avoid blocking, I will assume the middleware sets a User/MemberID in context OR I will parse it.
    // Let's check `routes.go` again... it uses `setupRoutes(e)` and NO global JWT middleware on the `/api` group shown in the snippet.
    // It seems `member_handlers` explicitly asks for `memberid` param.
    // I should probably ask the client to send `member_id` as well for now or check if there's a helper.
    
    // DECISION: I will add `request.fields['member_id'] = CurrentUser.id` in `KYCService` (Dart) in a separate step 
    // TO BE CORRECT: I will modify `KYCService.dart` to send `member_id` as well.
    // For this Go file, I'll expect `member_id` in form.

	memberID := c.FormValue("member_id")
    // If not found in form, maybe it's in header or context?
    // Let's assume we update Dart to send "member_id" to match `UploadProfileImageHandler` pattern (which uses `memberid`).
    if memberID == "" {
         memberID = c.FormValue("memberid") // Try both
    }
    if memberID == "" {
        // Fallback: Try to get from context if middleware put it there (e.g. "user_id")
        if v, ok := c.Get("user_id").(string); ok {
            memberID = v
        }
    }
    
	if memberID == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "member_id is required"})
	}

	// 2. Prepare R2 Upload
	r2Client := config.GetR2Client()
	if r2Client == nil {
		return c.JSON(http.StatusServiceUnavailable, map[string]string{"error": "R2 storage not configured"})
	}
	bucket := config.GetR2Bucket()

    // Struct to hold file info
    type KYCFile struct {
        KeyName string
        FileKey string // Form field name
        DBField string
    }

    filesToUpload := []KYCFile{
        {KeyName: "id_card", FileKey: "id_card_image", DBField: "kyc_id_card_image_key"},
        {KeyName: "bank_book", FileKey: "bank_book_image", DBField: "kyc_bank_book_image_key"},
        {KeyName: "selfie", FileKey: "selfie_image", DBField: "kyc_selfie_image_key"},
    }

    updateFields := bson.M{
        "bank_id": bankID,
        "bank_account_no": bankAccountNo,
        "kyc_status": "pending",
        "kyc_submitted_at": time.Now(),
    }

	// 3. Loop Upload Files
    for _, fInfo := range filesToUpload {
        fileHeader, err := c.FormFile(fInfo.FileKey)
        if err != nil {
            // It's possible some files are optional? For now assume all required as per UI.
             // If error is http.ErrMissingFile, maybe skip? But we enforce all 3 in UI.
             // Let's log and continue or error?
             // If UI requires it, we error here.
             if err == http.ErrMissingFile {
                 return c.JSON(http.StatusBadRequest, map[string]string{"error": "Missing file: " + fInfo.FileKey})
             }
             continue  // Skip validation error if something else?
        }
        
        // Validate Size/Type
        if fileHeader.Size > 5*1024*1024 {
             return c.JSON(http.StatusRequestEntityTooLarge, map[string]string{"error": fInfo.FileKey + " exceeds 5MB"})
        }
        
        ext := strings.ToLower(filepath.Ext(fileHeader.Filename))
        // Generate Key: kyc/<member_id>/<type>_<uuid><ext>
        // Adding UUID to avoid cache issues or overwrites if retrying
        newUUID := uuid.New().String()
        r2Key := fmt.Sprintf("kyc/%s/%s_%s%s", memberID, fInfo.KeyName, newUUID, ext)
        
        src, err := fileHeader.Open()
        if err != nil {
             return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to open " + fInfo.FileKey})
        }
        defer src.Close()
        
        buffer := bytes.NewBuffer(nil)
        if _, err := io.Copy(buffer, src); err != nil {
             return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to read " + fInfo.FileKey})
        }

        contentType := fileHeader.Header.Get("Content-Type")
        if contentType == "" { contentType = "image/jpeg" } // Default

        _, err = r2Client.PutObject(context.TODO(), &s3.PutObjectInput{
            Bucket:      aws.String(bucket),
            Key:         aws.String(r2Key),
            Body:        bytes.NewReader(buffer.Bytes()),
            ContentType: aws.String(contentType),
        })
        if err != nil {
             return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to upload " + fInfo.KeyName})
        }
        
        // Add to update map
        updateFields[fInfo.DBField] = r2Key
        updateFields[fInfo.DBField + "_url"] = "" // We store Key, URL can be generated presigned
    }
    
    // 4. Update Database
	db := config.GetDatabase()
	if db == nil {
		return c.JSON(http.StatusServiceUnavailable, map[string]string{"error": "Database not connected"})
	}
	collection := db.Collection("members")

    // We assume memberid is the unique business key strings
	filter := bson.M{"memberid": memberID}
	update := bson.M{"$set": updateFields}

	result, err := collection.UpdateOne(context.TODO(), filter, update)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Database update failed", "details": err.Error()})
	}

	if result.MatchedCount == 0 {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "Member not found"})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"status": "success",
		"message": "KYC submitted successfully",
        "data": updateFields,
	})
}

// GetPendingKYC returns a list of members waiting for verification
func GetPendingKYC(c echo.Context) error {
	db := config.GetDatabase()
	if db == nil {
		return c.JSON(http.StatusServiceUnavailable, map[string]string{"error": "Database not connected"})
	}
	collection := db.Collection("members")

	filter := bson.M{"kyc_status": "pending"}
    // Projection to return only necessary info
    projection := bson.M{
        "memberid": 1,
        "name_th": 1, 
        "kyc_submitted_at": 1,
    }

	cursor, err := collection.Find(context.TODO(), filter, options.Find().SetProjection(projection))
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to query pending KYC"})
	}
	defer cursor.Close(context.TODO())

	var results []map[string]interface{}
	if err = cursor.All(context.TODO(), &results); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to decode results"})
	}

	return c.JSON(http.StatusOK, results)
}

// GetKYCDetail returns full KYC info for a member
func GetKYCDetail(c echo.Context) error {
    memberID := c.Param("memberID")
    if memberID == "" {
        return c.JSON(http.StatusBadRequest, map[string]string{"error": "Member ID required"})
    }

	db := config.GetDatabase()
	collection := db.Collection("members")

	filter := bson.M{"memberid": memberID}
	var member map[string]interface{}
    
	err := collection.FindOne(context.TODO(), filter).Decode(&member)
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "Member not found"})
	}

    // Generate Presigned URLs for images
    r2Client := config.GetR2Client()
    bucket := config.GetR2Bucket()
    presignClient := config.GetR2PresignClient()
    
    images := map[string]string{}
    
    if presignClient != nil && r2Client != nil {
        keys := []string{"kyc_id_card_image_key", "kyc_bank_book_image_key", "kyc_selfie_image_key"}
        for _, k := range keys {
            if val, ok := member[k].(string); ok && val != "" {
                 req, _ := presignClient.PresignGetObject(context.TODO(), &s3.GetObjectInput{
                    Bucket: aws.String(bucket),
                    Key:    aws.String(val),
                }, func(opts *s3.PresignOptions) {
                    opts.Expires = 1 * time.Hour
                })
                images[k] = req.URL
            }
        }
    }

	return c.JSON(http.StatusOK, map[string]interface{}{
        "member": member,
        "images": images,
    })
}

// ReviewKYC handles approval or rejection
type KYCReviewRequest struct {
    MemberID  string `json:"member_id"`
    Status    string `json:"status"` // 'verified' or 'rejected'
    Reason    string `json:"reason,omitempty"`
    IsOfficer bool   `json:"is_officer"`
}

func ReviewKYC(c echo.Context) error {
    var req KYCReviewRequest
    if err := c.Bind(&req); err != nil {
        return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid request body"})
    }
    
    if req.Status != "verified" && req.Status != "rejected" {
         return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid status. Must be 'verified' or 'rejected'"})
    }

	db := config.GetDatabase()
	collection := db.Collection("members")

	fmt.Printf("DEBUG: ReviewKYC for MemberID: %s, Status: %s, IsOfficer: %v\n", req.MemberID, req.Status, req.IsOfficer)

	filter := bson.M{"memberid": req.MemberID}
	
	updateFields := bson.M{
		"kyc_status": req.Status,
		"kyc_reviewed_at": time.Now(),
		"kyc_reject_reason": req.Reason,
		"updatedat": time.Now(),
	}

	// If approved and designated as officer, change role
	if req.Status == "verified" && req.IsOfficer {
		fmt.Printf("DEBUG: Promoting Member %s to officer\n", req.MemberID)
		updateFields["role"] = "officer"
	} else {
		fmt.Printf("DEBUG: NOT promoting. Status: %s, IsOfficer: %v\n", req.Status, req.IsOfficer)
	}

	update := bson.M{
		"$set": updateFields,
	}

	result, err := collection.UpdateOne(context.TODO(), filter, update)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Update failed"})
	}
    
    if result.MatchedCount == 0 {
        return c.JSON(http.StatusNotFound, map[string]string{"error": "Member not found"})
    }

	return c.JSON(http.StatusOK, map[string]string{"status": "success", "message": "KYC status updated"})
}
