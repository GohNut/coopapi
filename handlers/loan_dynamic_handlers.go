package handlers

import (
    "context"
    "fmt"
    "net/http"
    "time"

    "github.com/google/uuid"
    "github.com/labstack/echo/v4"
    "go.mongodb.org/mongo-driver/bson"
    "go.mongodb.org/mongo-driver/mongo"
    "go.mongodb.org/mongo-driver/mongo/options"

    "loan-dynamic-api/config"
)

// DynamicGatewayRequest represents dynamic request from frontend
type DynamicGatewayRequest struct {
    Database   string                 `json:"database"`
    Collection string                 `json:"collection"`
    Data       map[string]interface{} `json:"data"`
    Upsert     bool                   `json:"upsert"`
}

// DynamicGatewayGetRequest represents GET request
type DynamicGatewayGetRequest struct {
    Database string                 `json:"database"`
    Collection string               `json:"collection"`
    Filter     map[string]interface{} `json:"filter"`
    Limit      int64                `json:"limit"`
    Skip       int64                `json:"skip"`
}

// DynamicGatewayUpdateRequest represents UPDATE request with filter
type DynamicGatewayUpdateRequest struct {
    Database   string                 `json:"database"`
    Collection string                 `json:"collection"`
    Filter     map[string]interface{} `json:"filter"`
    Data       map[string]interface{} `json:"data"`
    Upsert     bool                   `json:"upsert"`
}

// LoanDynamicCreate - สร้างคำขอสินเชื่อแบบ Dynamic
func LoanDynamicCreate(c echo.Context) error {
    // ตรวจสอบการเชื่อมต่อ
    if config.GetDatabase() == nil {
        return c.JSON(http.StatusServiceUnavailable, map[string]interface{}{
            "status":  "error",
            "code":    503,
            "message": "MongoDB Atlas is not connected",
        })
    }

    // Bind request body
    var req DynamicGatewayRequest
    if err := c.Bind(&req); err != nil {
        return c.JSON(http.StatusBadRequest, map[string]interface{}{
            "status":  "error",
            "code":    400,
            "message": "Invalid request body",
            "error":   err.Error(),
        })
    }

    // Validate required fields
    if req.Collection == "" {
        return c.JSON(http.StatusBadRequest, map[string]interface{}{
            "status":  "error",
            "code":    400,
            "message": "Collection name is required",
        })
    }

    // [New] Validate collection whitelist
    if !isCollectionAllowed(req.Collection) {
        return c.JSON(http.StatusForbidden, map[string]interface{}{
            "status":  "error",
            "code":    403,
            "message": "Collection not allowed",
        })
    }

    if req.Data == nil {
        return c.JSON(http.StatusBadRequest, map[string]interface{}{
            "status":  "error",
            "code":    400,
            "message": "Data field is required",
        })
    }

    // [New] Validate data size
    if err := validateDataSize(req.Data); err != nil {
        return c.JSON(http.StatusRequestEntityTooLarge, map[string]interface{}{
            "status":  "error",
            "code":    413,
            "message": err.Error(),
        })
    }

    // เตรียม database และ context
    ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()

    db := config.GetDatabase()
    if req.Database != "" {
        db = config.GetDatabase().Client().Database(req.Database)
    }

    // [New] KYC Check for Transactions
    if req.Collection == "deposit_transactions" {
        if err := checkTransactionKYC(req.Data); err != nil {
             return c.JSON(http.StatusForbidden, map[string]interface{}{
                "status":  "error",
                "code":    403,
                "message": err.Error(),
            })
        }
    }

    // [New] Duplicate Check for Members
    if req.Collection == "members" {
        memberID, _ := req.Data["memberid"].(string)
        appID, _ := req.Data["applicationid"].(string)
        
        filter := bson.M{
            "$or": []bson.M{
                {"memberid": memberID},
                {"applicationid": appID},
            },
        }
        
        var existing map[string]interface{}
        err := db.Collection("members").FindOne(ctx, filter).Decode(&existing)
        if err == nil {
            // Found existing member
            return c.JSON(http.StatusConflict, map[string]interface{}{
                "status":  "error",
                "code":    409,
                "message": "Member with this citizen ID or application ID already exists",
            })
        }
    }
    if req.Data["applicationid"] == nil {
        req.Data["applicationid"] = uuid.New().String()
    }
    if req.Data["createdat"] == nil {
        req.Data["createdat"] = time.Now()
    }
    if req.Data["updatedat"] == nil {
        req.Data["updatedat"] = time.Now()
    }

    // คำนวณค่างวดและยอดรวมสำหรับ loan applications
    if req.Collection == "loan_applications" {
        calculateAndAddLoanData(req.Data)
    }

    // บันทึกลง MongoDB
    collection := db.Collection(req.Collection)
    
    var result *mongo.InsertOneResult
    var err error

    if req.Upsert {
        // ถ้าใช้ upsert ให้ใช้ UpdateOne แทน
        filter := bson.M{"applicationid": req.Data["applicationid"]}
        update := bson.M{"$set": req.Data}
        opts := options.Update().SetUpsert(true)
        
        _, err = collection.UpdateOne(ctx, filter, update, opts)
        
        // Note: UpdateOne doesn't return InsertedID directly like InsertOne
        // We can just return success
    } else {
        // Insert ใหม่
        result, err = collection.InsertOne(ctx, req.Data)
    }

    if err != nil {
        return c.JSON(http.StatusInternalServerError, map[string]interface{}{
            "status":  "error",
            "code":    500,
            "message": "Failed to create loan data",
            "error":   err.Error(),
        })
    }

    response := map[string]interface{}{
        "status": "success",
        "code":   201,
        "message": "Loan data created successfully",
    }
    if result != nil {
        response["inserted_id"] = result.InsertedID
    }

    // เพิ่มข้อมูลเพิ่มเติมสำหรับ loan applications
    if req.Collection == "loan_applications" {
        if installment, ok := req.Data["installmentamount"].(float64); ok {
            response["installment_amount"] = installment
        }
        if total, ok := req.Data["totalpayment"].(float64); ok {
            response["total_payment"] = total
        }
        response["application_id"] = req.Data["applicationid"]
    }

    return c.JSON(http.StatusCreated, response)
}

// LoanDynamicGet - ดึงข้อมูลสินเชื่อแบบ Dynamic
func LoanDynamicGet(c echo.Context) error {
    // ตรวจสอบการเชื่อมต่อ
    if config.GetDatabase() == nil {
        return c.JSON(http.StatusServiceUnavailable, map[string]interface{}{
            "status":  "error",
            "code":    503,
            "message": "MongoDB Atlas is not connected",
        })
    }

    // Bind request body
    var req DynamicGatewayGetRequest
    if err := c.Bind(&req); err != nil {
        return c.JSON(http.StatusBadRequest, map[string]interface{}{
            "status":  "error",
            "code":    400,
            "message": "Invalid request body",
            "error":   err.Error(),
        })
    }

    // Validate required fields
    if req.Collection == "" {
        return c.JSON(http.StatusBadRequest, map[string]interface{}{
            "status":  "error",
            "code":    400,
            "message": "Collection name is required",
        })
    }

    // [New] Validate collection whitelist
    if !isCollectionAllowed(req.Collection) {
        return c.JSON(http.StatusForbidden, map[string]interface{}{
            "status":  "error",
            "code":    403,
            "message": "Collection not allowed",
        })
    }

    if req.Filter == nil {
        return c.JSON(http.StatusBadRequest, map[string]interface{}{
            "status":  "error",
            "code":    400,
            "message": "Filter is required",
        })
    }

    // ตั้งค่า query options
    opts := options.Find()
    if req.Limit > 0 {
        opts.SetLimit(req.Limit)
    }
    if req.Skip > 0 {
        opts.SetSkip(req.Skip)
    }
    opts.SetSort(bson.D{{"createdat", -1}})

    // Execute query
    ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()

    db := config.GetDatabase()
    if req.Database != "" {
        db = config.GetDatabase().Client().Database(req.Database)
    }

    collection := db.Collection(req.Collection)
    cursor, err := collection.Find(ctx, req.Filter, opts)
    if err != nil {
        return c.JSON(http.StatusInternalServerError, map[string]interface{}{
            "status":  "error",
            "code":    500,
            "message": "Failed to query documents",
            "error":   err.Error(),
        })
    }
    defer cursor.Close(ctx)

    // Decode results
    var results []bson.M
    if err := cursor.All(ctx, &results); err != nil {
        return c.JSON(http.StatusInternalServerError, map[string]interface{}{
            "status":  "error",
            "code":    500,
            "message": "Failed to decode documents",
            "error":   err.Error(),
        })
    }

    return c.JSON(http.StatusOK, map[string]interface{}{
        "status": "success",
        "code":   200,
        "count":  len(results),
        "data":   results,
    })
}

// LoanDynamicUpdate - อัปเดตข้อมูลสินเชื่อแบบ Dynamic
func LoanDynamicUpdate(c echo.Context) error {
    // ตรวจสอบการเชื่อมต่อ
    if config.GetDatabase() == nil {
        return c.JSON(http.StatusServiceUnavailable, map[string]interface{}{
            "status":  "error",
            "code":    503,
            "message": "MongoDB Atlas is not connected",
        })
    }

    // Bind request body
    var req DynamicGatewayUpdateRequest
    if err := c.Bind(&req); err != nil {
        return c.JSON(http.StatusBadRequest, map[string]interface{}{
            "status":  "error",
            "code":    400,
            "message": "Invalid request body",
            "error":   err.Error(),
        })
    }

    // Validate required fields
    if req.Collection == "" {
        return c.JSON(http.StatusBadRequest, map[string]interface{}{
            "status":  "error",
            "code":    400,
            "message": "Collection name is required",
        })
    }

    // [New] Validate collection whitelist
    if !isCollectionAllowed(req.Collection) {
        return c.JSON(http.StatusForbidden, map[string]interface{}{
            "status":  "error",
            "code":    403,
            "message": "Collection not allowed",
        })
    }

    if req.Filter == nil {
        return c.JSON(http.StatusBadRequest, map[string]interface{}{
            "status":  "error",
            "code":    400,
            "message": "Filter is required",
        })
    }

    if req.Data == nil {
        return c.JSON(http.StatusBadRequest, map[string]interface{}{
            "status":  "error",
            "code":    400,
            "message": "Data field is required",
        })
    }

    // [New] Validate data size
    if err := validateDataSize(req.Data); err != nil {
        return c.JSON(http.StatusRequestEntityTooLarge, map[string]interface{}{
            "status":  "error",
            "code":    413,
            "message": err.Error(),
        })
    }

    // เพิ่ม updated timestamp
    req.Data["updatedat"] = time.Now()

    // Build update document
    update := bson.M{
        "$set": req.Data,
    }

    // Perform update
    ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()

    db := config.GetDatabase()
    if req.Database != "" {
        db = config.GetDatabase().Client().Database(req.Database)
    }

    collection := db.Collection(req.Collection)
    opts := options.Update().SetUpsert(req.Upsert)

    result, err := collection.UpdateOne(ctx, req.Filter, update, opts)
    if err != nil {
        return c.JSON(http.StatusInternalServerError, map[string]interface{}{
            "status":  "error",
            "code":    500,
            "message": "Failed to update document",
            "error":   err.Error(),
        })
    }

    return c.JSON(http.StatusOK, map[string]interface{}{
        "status":         "success",
        "code":           200,
        "matched_count":  result.MatchedCount,
        "modified_count": result.ModifiedCount,
        "upserted_id":    result.UpsertedID,
    })
}

// LoanDynamicDelete - ลบข้อมูลสินเชื่อแบบ Dynamic
func LoanDynamicDelete(c echo.Context) error {
    // ตรวจสอบการเชื่อมต่อ
    if config.GetDatabase() == nil {
        return c.JSON(http.StatusServiceUnavailable, map[string]interface{}{
            "status":  "error",
            "code":    503,
            "message": "MongoDB Atlas is not connected",
        })
    }

    // Bind request body
    var req DynamicGatewayGetRequest
    if err := c.Bind(&req); err != nil {
        return c.JSON(http.StatusBadRequest, map[string]interface{}{
            "status":  "error",
            "code":    400,
            "message": "Invalid request body",
            "error":   err.Error(),
        })
    }

    // Validate required fields
    if req.Collection == "" {
        return c.JSON(http.StatusBadRequest, map[string]interface{}{
            "status":  "error",
            "code":    400,
            "message": "Collection name is required",
        })
    }

    if req.Filter == nil {
        return c.JSON(http.StatusBadRequest, map[string]interface{}{
            "status":  "error",
            "code":    400,
            "message": "Filter is required",
        })
    }

    // Perform delete
    ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()

    db := config.GetDatabase()
    if req.Database != "" {
        db = config.GetDatabase().Client().Database(req.Database)
    }

    collection := db.Collection(req.Collection)
    result, err := collection.DeleteOne(ctx, req.Filter)

    if err != nil {
        return c.JSON(http.StatusInternalServerError, map[string]interface{}{
            "status":  "error",
            "code":    500,
            "message": "Failed to delete document",
            "error":   err.Error(),
        })
    }

    return c.JSON(http.StatusOK, map[string]interface{}{
        "status":        "success",
        "code":          200,
        "deleted_count": result.DeletedCount,
    })
}

// Helper function to calculate loan data
func calculateAndAddLoanData(data map[string]interface{}) {
    // คำนวณค่างวดและยอดรวมถ้ามีข้อมูลครบ
    if requestAmount, ok := data["requestamount"].(float64); ok {
        if interestRate, ok := data["interestrate"].(float64); ok {
            if requestTerm, ok := data["requestterm"].(float64); ok { // JSON often decodes numbers as float64
                // Convert float64 term to int
                term := int(requestTerm)
                
                installmentAmount := calculateInstallment(requestAmount, interestRate, term)
                totalPayment := installmentAmount * float64(term)
                totalInterest := totalPayment - requestAmount

                data["installmentamount"] = installmentAmount
                data["totalpayment"] = totalPayment
                data["totalinterest"] = totalInterest
            }
        }
    }
}

// Helper function to calculate installment
func calculateInstallment(amount float64, interestRate float64, term int) float64 {
    // Flat Rate Formula
    totalInterest := amount * (interestRate / 100) * (float64(term) / 12)
    totalPayment := amount + totalInterest
    return totalPayment / float64(term)
}

// Helper function to check KYC status for transactions
func checkTransactionKYC(data map[string]interface{}) error {
    // 1. Extract info
    accountID, _ := data["accountid"].(string)
    txType, _ := data["type"].(string)
    status, _ := data["status"].(string)

    if accountID == "" {
        return nil // Skip if no account ID (should usually fail elsewhere or be irrelevant)
    }

    // 2. Determine if check is needed
    shouldCheck := false
    
    switch txType {
    case "withdrawal", "transfer_out", "payment", "pay":
        shouldCheck = true
    case "deposit":
        // Only check pending deposits (User initiated)
        // Officer deposit might be completed directly (if implemented that way), or system refund
        if status == "pending" {
            shouldCheck = true
        }
    }

    if !shouldCheck {
        return nil
    }

    // 3. Connect DB
    db := config.GetDatabase()
    if db == nil {
        return fmt.Errorf("database connection failed")
    }

    // 4. Find Member ID from Account
    var account map[string]interface{}
    err := db.Collection("deposit_accounts").FindOne(context.Background(), bson.M{"accountid": accountID}).Decode(&account)
    if err != nil {
        return fmt.Errorf("account not found")
    }

    memberID, _ := account["memberid"].(string)
    if memberID == "" {
        return fmt.Errorf("member not found for this account")
    }

    // 5. Check Member KYC Status
    var member map[string]interface{}
    err = db.Collection("members").FindOne(context.Background(), bson.M{"memberid": memberID}).Decode(&member)
    if err != nil {
        return fmt.Errorf("member profile not found")
    }

    kycStatus, _ := member["kyc_status"].(string)
    if kycStatus != "verified" {
        return fmt.Errorf("KYC verification required for this transaction")
    }

    return nil
}
