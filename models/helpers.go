package models

import "go.mongodb.org/mongo-driver/bson/primitive"

// ObjectID converts a string to a MongoDB ObjectID without the need of error checking
// (placed here so the database package is not required by the controllers package)
func ObjectID(ID string) primitive.ObjectID {
	id, err := primitive.ObjectIDFromHex(ID)
	if err != nil {
		return primitive.NilObjectID
	}
	return id
}
