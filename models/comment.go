package models

import (
	"context"
	"fmt"
	"forza-garage/database"
	"forza-garage/helpers"
	"forza-garage/lookups"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

// Comment is the "interface" used for client communication
// optimistic locking not required here
type Comment struct {
	ID           primitive.ObjectID `json:"id" bson:"_id,omitempty"`                  // required for comments
	ProfileID    primitive.ObjectID `json:"profileId" bson:"profileId,omitempty"`     // required for comments
	ProfileType  string             `json:"profileType" bson:"profileType,omitempty"` // required for comments
	CreatedTS    time.Time          `json:"createdTS" bson:"createdTS,omitempty"`     // required for replies
	CreatedID    primitive.ObjectID `json:"createdID" bson:"createdID"`
	CreatedName  string             `json:"createdName" bson:"createdName"`
	ModifiedTS   time.Time          `json:"modifiedTS" bson:"modifiedTS,omitempty"` // edited if present
	ModifiedID   primitive.ObjectID `json:"modifiedID" bson:"modifiedID,omitempty"` // maybe used to flag "edited by admin"
	ModifiedName string             `json:"modifiedName" bson:"modifiedName,omitempty"`
	Rating       float32            `json:"rating" bson:"rating"`         // calculated by the voting function & persisted
	RatingSort   float32            `json:"ratingSort" bson:"ratingSort"` // calculated by the voting function & persisted (lowerBound)
	StatusCode   int32              `json:"statusCode" bson:"statusCD"`
	StatusText   string             `json:"gameCode" bson:"-"`
	StatusTS     time.Time          `json:"statusTS" bson:"statusTS"`
	StatusID     primitive.ObjectID `json:"statusID" bson:"statusID"`
	StatusName   string             `json:"statusName" bson:"statusName"`
	Pinned       bool               `json:"pinned" bson:"pinned"`
	Comment      string             `json:"comment" bson:"comment"`
	Replies      []Comment          `json:"replies" bson:"replies,omitempty"`
}

// CommentModel provides the logic to the interface and access to the database
type CommentModel struct {
	Collection *mongo.Collection
	// Gewisse Informationen kommen vom User-Model, die werden hier referenziert
	// somit muss das nicht der Controller machen
	GetUserName    func(ID string) (string, error)
	GetCredentials func(userId string, loadFriendlist bool) *Credentials
}

// Validate checks given values and sets defaults where applicable (immutable)
func (m CommentModel) Validate(comment Comment) (*Comment, error) {

	cleaned := comment

	// hier kann eine "Zensur-Func" afgerufen werden
	cleaned.Comment = strings.TrimSpace(cleaned.Comment)

	if cleaned.Comment == "" {
		return nil, ErrCommentEmpty
	}

	return &cleaned, nil
}

// Create adds a new Comment or Response
func (m CommentModel) Create(comment *Comment, userID string) (string, error) {

	// Validate called by controller

	// set common fields
	now := time.Now()
	comment.CreatedTS = now
	comment.CreatedID = database.ObjectID(userID)
	userName, err := m.GetUserName(userID)
	if err != nil {
		// Fachlicher Fehler oder bereits wrapped
		return "", err
	}
	comment.CreatedName = userName
	comment.Rating = 0
	comment.RatingSort = 0
	comment.StatusCode = lookups.CommentStatusPending
	comment.StatusTS = now
	comment.StatusID = comment.CreatedID
	comment.StatusName = comment.CreatedName

	if comment.ID == primitive.NilObjectID {
		// new comment
		comment.ID = primitive.NewObjectID()

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel() // nach 10 Sekunden abbrechen

		res, err := m.Collection.InsertOne(ctx, comment)
		if err != nil {
			return "", helpers.WrapError(err, helpers.FuncName()) // primitive.NilObjectID.Hex() ? probly useless
		}

		return res.InsertedID.(primitive.ObjectID).Hex(), nil
	} else {
		// reply - push array
		// (update of an existing comment's document, but not that comment's M-Fields)

		// ID set by controller

		fmt.Println("test")

		return "", nil
	}

}
