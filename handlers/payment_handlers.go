package handlers

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"

	"loan-dynamic-api/config"
)

// InternalTransferRequest represents the payload for internal transfer
type InternalTransferRequest struct {
	SourceAccountID StringOrMap `json:"source_account_id"` // Support both direct string or filter map
	DestAccountID   StringOrMap `json:"dest_account_id"`
	Amount          float64     `json:"amount"`
	Description     string      `json:"description"`
}

type StringOrMap interface{}

// InternalTransferResponse represents the response for internal transfer
type InternalTransferResponse struct {
	Status        string `json:"status"`
	TransactionID string `json:"transaction_id"`
	Message       string `json:"message"`
}

// PerformInternalTransfer handles money transfer between two accounts
func PerformInternalTransfer(c echo.Context) error {
	var req map[string]interface{}
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid request body"})
	}

	sourceAccountID, _ := req["source_account_id"].(string)
	destAccountID, _ := req["dest_account_id"].(string)
	amount, _ := req["amount"].(float64)
	description, _ := req["description"].(string)

	if sourceAccountID == "" || destAccountID == "" || amount <= 0 {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "source_account_id, dest_account_id and valid amount are required"})
	}

	db := config.GetDatabase()
	if db == nil {
		return c.JSON(http.StatusServiceUnavailable, map[string]string{"error": "Database not connected"})
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// 1. Get Source Account
	var sourceAccount bson.M
	err := db.Collection("deposit_accounts").FindOne(ctx, bson.M{"accountid": sourceAccountID}).Decode(&sourceAccount)
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "Source account not found"})
	}

	// 2. Check Balance
	sourceBalance, _ := sourceAccount["balance"].(float64)
	if sourceBalance < amount {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Insufficient balance"})
	}

	// 3. Get Destination Account
	var destAccount bson.M
	err = db.Collection("deposit_accounts").FindOne(ctx, bson.M{"accountid": destAccountID}).Decode(&destAccount)
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "Destination account not found"})
	}

	destBalance, _ := destAccount["balance"].(float64)

	// Since we are not using full transactions to keep it simple and portable (Atlas free tier might not support or setup is complex), 
	// we use individual updates. For production, please use mongo.Session if supported.
	
	session, err := db.Client().StartSession()
	if err == nil {
		defer session.EndSession(ctx)
		err = mongo.WithSession(ctx, session, func(sc mongo.SessionContext) error {
			if err := session.StartTransaction(); err != nil {
				return err
			}

			// A. Deduct from Source
			_, err = db.Collection("deposit_accounts").UpdateOne(sc, 
				bson.M{"accountid": sourceAccountID}, 
				bson.M{"$inc": bson.M{"balance": -amount}})
			if err != nil {
				session.AbortTransaction(sc)
				return err
			}

			// B. Add to Destination
			_, err = db.Collection("deposit_accounts").UpdateOne(sc, 
				bson.M{"accountid": destAccountID}, 
				bson.M{"$inc": bson.M{"balance": amount}})
			if err != nil {
				session.AbortTransaction(sc)
				return err
			}

			// C. Create Transactions
			now := time.Now()
			sourceTxID := fmt.Sprintf("TXN-OUT-%d", now.UnixNano())
			destTxID := fmt.Sprintf("TXN-IN-%d", now.UnixNano())

			sourceTx := bson.M{
				"transactionid": sourceTxID,
				"accountid":     sourceAccountID,
				"type":          "transfer_out",
				"amount":        amount,
				"balanceafter":  sourceBalance - amount,
				"datetime":       now,
				"description":   fmt.Sprintf("%s (โอนให้ %s)", description, destAccount["accountname"]),
				"referenceno":   destAccountID,
				"status":        "completed",
			}

			destTx := bson.M{
				"transactionid": destTxID,
				"accountid":     destAccountID,
				"type":          "transfer_in",
				"amount":        amount,
				"balanceafter":  destBalance + amount,
				"datetime":       now,
				"description":   fmt.Sprintf("%s (รับจาก %s)", description, sourceAccount["accountname"]),
				"referenceno":   sourceAccountID,
				"status":        "completed",
			}

			_, err = db.Collection("deposit_transactions").InsertMany(sc, []interface{}{sourceTx, destTx})
			if err != nil {
				session.AbortTransaction(sc)
				return err
			}

			// // D. Notification for Receiver
			// receiverMemberID, _ := destAccount["memberid"].(string)
			// if receiverMemberID != "" {
			// 	notification := bson.M{
			// 		"notificationid": fmt.Sprintf("NOTI-%d", now.UnixNano()),
			// 		"memberid":       receiverMemberID,
			// 		"title":          "เงินเข้าบัญชี",
			// 		"message":        fmt.Sprintf("คุณได้รับเงินโอนจำนวน %.2f บาท จาก %s", amount, sourceAccount["accountname"]),
			// 		"type":           "success",
			// 		"read":           false,
			// 		"createdat":      now,
			// 	}
			// 	db.Collection("notifications").InsertOne(sc, notification)
			// }

			return session.CommitTransaction(sc)
		})
	}

	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Transfer failed", "details": err.Error()})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"status":  "success",
		"message": "Transfer completed successfully",
	})
}
