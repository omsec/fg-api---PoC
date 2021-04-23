package models

import (
	"context"
	"forza-garage/apperror"
	"forza-garage/database"
	"forza-garage/helpers"
	"forza-garage/lookups"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

// PROBLEME/FRAGEN
// Wie kann Paging gemacht werden? (load 10 more) - Offset, laufende ID ??
// https://www.codementor.io/@arpitbhayani/fast-and-efficient-pagination-in-mongodb-9095flbqr

// Comment is the "interface" used for client communication
// optimistic locking not required here
type Comment struct {
	ID           primitive.ObjectID `json:"id" bson:"_id"`                            // comment or reply ID
	ProfileID    primitive.ObjectID `json:"profileId" bson:"profileId,omitempty"`     // required for comments
	ProfileType  *string            `json:"profileType" bson:"profileType,omitempty"` // required for comments
	CreatedTS    time.Time          `json:"createdTS" bson:"-"`                       // extracted from OID
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
	Pinned       *bool              `json:"pinned" bson:"pinned,omitempty"`
	Comment      string             `json:"comment" bson:"comment"`
	Replies      []Comment          `json:"replies" bson:"replies,omitempty"` // applies to GET-requests only
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
		// new reply - push array
		// (update of an existing comment's document, but not that comment's M-Fields)

		// remove fields which are not necessary for saving replies
		id := comment.ID                     // save original ID of the parent comment for update
		comment.ID = primitive.NewObjectID() // generate UID for the reply
		comment.ProfileID = primitive.NilObjectID
		comment.ProfileType = nil
		comment.Pinned = nil // by convention, answers can't be pinged
		comment.Replies = nil

		// ID set by controller
		filter := bson.D{{Key: "_id", Value: id}}
		fields := bson.D{
			{Key: "$push", Value: bson.D{
				{Key: "replies", Value: bson.D{
					{Key: "$each", Value: bson.A{comment}},
					{Key: "$position", Value: 0},
				}},
			}},
		}

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel() // nach 10 Sekunden abbrechen

		result, err := m.Collection.UpdateOne(ctx, filter, fields)
		if err != nil {
			return "", helpers.WrapError(err, helpers.FuncName())
		}

		if result.MatchedCount == 0 {
			return "", apperror.ErrNoData // document might have been deleted
		}

		return comment.ID.Hex(), nil
	}

}
