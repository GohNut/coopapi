package handlers

import (
	"encoding/json"
	"fmt"
)

// อนุญาตเฉพาะ collection ที่เกี่ยวข้องกับระบบสินเชื่อ
var allowedLoanCollections = map[string]bool{
	"loan_applications":    true,
	"loan_products":        true,
	"loan_tracking":        true,
	"loan_documents":       true,
	"loan_payments":        true,
	"deposit_accounts":     true,
	"deposit_transactions": true,
	"members":              true,
	"share_accounts":       true,
	"share_transactions":   true,
	"dividend_rates":       true,
	"dividend_payments":    true,
}

// Check if collection is allowed
func isCollectionAllowed(collection string) bool {
	return allowedLoanCollections[collection]
}

// ตรวจสอบขนาดข้อมูล
func validateDataSize(data map[string]interface{}) error {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return err
	}

	// 16MB limit (MongoDB document size limit)
	// Using slightly less than 16MB to be safe, e.g., 15MB
	const limit = 15 * 1024 * 1024

	if len(jsonData) > limit {
		return fmt.Errorf("data size exceeds limit")
	}
	return nil
}
