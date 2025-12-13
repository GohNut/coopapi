package config

import (
    "context"
    "crypto/tls"
    "fmt"
    "os"
    "time"

    "go.mongodb.org/mongo-driver/mongo"
    "go.mongodb.org/mongo-driver/mongo/options"
)

var (
    atlasClient *mongo.Client
    atlasDB     *mongo.Database
)

// InitMongoAtlas เชื่อมต่อ MongoDB Atlas
func InitMongoAtlas() error {
	if atlasClient != nil {
		return nil
	}

	uri := os.Getenv("MONGODB_URI")
	if uri == "" {
		return fmt.Errorf("MONGODB_URI not set")
	}

    ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()

    clientOptions := options.Client().ApplyURI(uri)
    
    // Skip TLS verification for development if certificate is missing
    clientOptions.SetTLSConfig(&tls.Config{InsecureSkipVerify: true})

    client, err := mongo.Connect(ctx, clientOptions)
    if err != nil {
        return fmt.Errorf("failed to connect to MongoDB Atlas: %w", err)
    }

    if err := client.Ping(ctx, nil); err != nil {
        return fmt.Errorf("failed to ping MongoDB Atlas: %w", err)
    }

    atlasClient = client

    dbName := os.Getenv("MONGODB_DB")
    if dbName == "" {
        dbName = "coop_digital"
    }
    atlasDB = client.Database(dbName)

    fmt.Printf("Connected to MongoDB Atlas successfully (DB: %s)\n", dbName)
    return nil
}

// GetDatabase ดึง database instance
func GetDatabase() *mongo.Database {
    return atlasDB
}

// getDatabase - เลือก database ตาม request หรือใช้ default
func getDatabase(dbName string) *mongo.Database {
    if dbName != "" && atlasClient != nil {
        return atlasClient.Database(dbName)
    }
    return atlasDB
}

// DisconnectMongoAtlas ปิดการเชื่อมต่อ
func DisconnectMongoAtlas() {
    if atlasClient != nil {
        ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
        defer cancel()
        
        if err := atlasClient.Disconnect(ctx); err != nil {
            fmt.Printf("Error disconnecting from MongoDB Atlas: %v\n", err)
        } else {
            fmt.Println("Disconnected from MongoDB Atlas")
        }
    }
}
