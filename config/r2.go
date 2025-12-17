package config

import (
	"context"
	"fmt"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

var (
	r2Client        *s3.Client
	r2PresignClient *s3.PresignClient
	r2Bucket        string
)

// InitR2 initializes the connection to Cloudflare R2
func InitR2() error {
	accountId := os.Getenv("R2_ACCOUNT_ID")
	accessKey := os.Getenv("R2_ACCESS_KEY_ID")
	secretKey := os.Getenv("R2_SECRET_ACCESS_KEY")
	bucketName := os.Getenv("R2_BUCKET_NAME")

	if accountId == "" || accessKey == "" || secretKey == "" || bucketName == "" {
		return fmt.Errorf("R2 configuration missing (R2_ACCOUNT_ID, R2_ACCESS_KEY_ID, R2_SECRET_ACCESS_KEY, R2_BUCKET_NAME)")
	}

	r2Bucket = bucketName

	r2Endpoint := fmt.Sprintf("https://%s.r2.cloudflarestorage.com", accountId)

	cfg, err := config.LoadDefaultConfig(context.TODO(),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(accessKey, secretKey, "")),
		config.WithRegion("auto"),
	)
	if err != nil {
		return fmt.Errorf("unable to load SDK config: %v", err)
	}

	r2Client = s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(r2Endpoint)
	})

	r2PresignClient = s3.NewPresignClient(r2Client)

	fmt.Println("R2 Client initialized successfully")
	return nil
}

// GetR2Client returns the S3 client for R2
func GetR2Client() *s3.Client {
	return r2Client
}

// GetR2PresignClient returns the Presign client
func GetR2PresignClient() *s3.PresignClient {
	return r2PresignClient
}

// GetR2Bucket returns the configured bucket name
func GetR2Bucket() string {
	return r2Bucket
}
