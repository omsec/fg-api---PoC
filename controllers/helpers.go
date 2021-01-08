package controllers

// Created is the standard response for new items
type Created struct {
	ID string `json:"id" bson:"_id"`
}
