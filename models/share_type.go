package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type ShareType struct {
	ID        primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	Name      string             `bson:"name" json:"name"`
	Price     float64            `bson:"price" json:"price"`
	Status    string             `bson:"status" json:"status"` // active, inactive
	CreatedAt time.Time          `bson:"created_at" json:"created_at"`
	UpdatedAt time.Time          `bson:"updated_at" json:"updated_at"`
}
