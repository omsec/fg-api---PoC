package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// Header is used as an embedded type for an object's meta-info
type Header struct {
	CreatedTS    time.Time          `json:"createdTS" bson:"-"` // CreatedTS is read from Mongo's ObjectID
	CreatedID    primitive.ObjectID `json:"createdID" bson:"createdID"`
	CreatedName  string             `json:"createdName" bson:"createdName"`
	ModifiedTS   time.Time          `json:"modifiedTS" bson:"modifiedTS,omitempty"` // edited if present
	ModifiedID   primitive.ObjectID `json:"modifiedID" bson:"modifiedID,omitempty"` // maybe used to flag "edited by admin"
	ModifiedName string             `json:"modifiedName" bson:"modifiedName,omitempty"`
	Rating       float32            `json:"rating" bson:"rating"`       // calculated & persisted
	TouchedTS    time.Time          `json:"touchedTS" bson:"touchedTS"` // de-norm of many sources (maybe nested or referenced)
	RecVer       int64              `json:"recVer" bson:"recVer"`       // optimistic locking
}

// SmallHeader is used for embedded content, such as comments or file references (arrays)
type SmallHeader struct {
	CreatedTS   time.Time          `json:"createdTS" bson:"createdTS"`
	CreatedID   primitive.ObjectID `json:"createdID" bson:"createdID"`
	CreatedName string             `json:"createdName" bson:"createdName"`
	ModifiedTS  time.Time          `json:"modifiedTS" bson:"modifiedTS,omitempty"`
	Rating      float32            `json:"rating" bson:"rating"` // calculated & persisted (sorting, usually not shown in clients)
}
