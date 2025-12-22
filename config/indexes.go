package config

import (
    "context"
    "fmt"
    "time"

    "go.mongodb.org/mongo-driver/bson"
    "go.mongodb.org/mongo-driver/mongo"
    "go.mongodb.org/mongo-driver/mongo/options"
)

// EnsureIndexes creates required indexes for the loan system
func EnsureIndexes() error {
    db := GetDatabase()
    if db == nil {
        return fmt.Errorf("database not connected")
    }

    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()

    // 1. loan_applications Indexes
    loanAppColl := db.Collection("loan_applications")
    loanAppIndexes := []mongo.IndexModel{
        {
            Keys: bson.D{{"memberid", 1}, {"email", 1}},
        },
        {
            Keys: bson.D{{"status", 1}},
        },
        {
            Keys: bson.D{{"requestdate", -1}},
        },
        {
            Keys:    bson.D{{"applicationid", 1}},
            Options: options.Index().SetUnique(true),
        },
        {
            Keys: bson.D{{"productid", 1}},
        },
        // Dynamic field indexes
        {
            Keys: bson.D{{"applicantinfo.mobile", 1}},
        },
    }

    if _, err := loanAppColl.Indexes().CreateMany(ctx, loanAppIndexes); err != nil {
        return fmt.Errorf("failed to create indexes for loan_applications: %w", err)
    }

    // 2. loan_products Indexes
    loanProdColl := db.Collection("loan_products")
    loanProdIndexes := []mongo.IndexModel{
        {
            Keys:    bson.D{{"productid", 1}},
            Options: options.Index().SetUnique(true),
        },
        {
            Keys: bson.D{{"maxamount", 1}},
        },
        {
            Keys: bson.D{{"interestrate", 1}},
        },
    }

    if _, err := loanProdColl.Indexes().CreateMany(ctx, loanProdIndexes); err != nil {
        return fmt.Errorf("failed to create indexes for loan_products: %w", err)
    }

    // 3. members Indexes
    memberColl := db.Collection("members")
    memberIndexes := []mongo.IndexModel{
        {
            Keys:    bson.D{{"memberid", 1}},
            Options: options.Index().SetUnique(true),
        },
        {
            Keys:    bson.D{{"applicationid", 1}},
            Options: options.Index().SetUnique(true),
        },
        {
            Keys: bson.D{{"mobile", 1}},
        },
        {
            Keys: bson.D{{"created_at", -1}},
        },
    }

    if _, err := memberColl.Indexes().CreateMany(ctx, memberIndexes); err != nil {
        return fmt.Errorf("failed to create indexes for members: %w", err)
    }

    // 4. deposit_accounts Indexes
    accColl := db.Collection("deposit_accounts")
    accIndexes := []mongo.IndexModel{
        {
            Keys:    bson.D{{"accountid", 1}},
            Options: options.Index().SetUnique(true),
        },
        {
            Keys:    bson.D{{"accountnumber", 1}},
            Options: options.Index().SetUnique(true),
        },
        {
            Keys: bson.D{{"memberid", 1}},
        },
    }

    if _, err := accColl.Indexes().CreateMany(ctx, accIndexes); err != nil {
        return fmt.Errorf("failed to create indexes for deposit_accounts: %w", err)
    }

    // 5. deposit_transactions Indexes
    txColl := db.Collection("deposit_transactions")
    txIndexes := []mongo.IndexModel{
        {
            Keys:    bson.D{{"transactionid", 1}},
            Options: options.Index().SetUnique(true),
        },
        {
            Keys: bson.D{{"accountid", 1}},
        },
        {
            Keys: bson.D{{"status", 1}},
        },
        {
            Keys: bson.D{{"datetime", -1}},
        },
    }

    if _, err := txColl.Indexes().CreateMany(ctx, txIndexes); err != nil {
        return fmt.Errorf("failed to create indexes for deposit_transactions: %w", err)
    }

    fmt.Println("Indexes ensured successfully")
    return nil
}
