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
	"go.mongodb.org/mongo-driver/mongo/options"
)

// PROBLEME/FRAGEN
// Wie kann Paging gemacht werden? (load 10 more) - Offset, laufende ID ??
// https://www.codementor.io/@arpitbhayani/fast-and-efficient-pagination-in-mongodb-9095flbqr

// Comment is the "interface" used for client communication
// optimistic locking not required here
type Comment struct {
	ID          primitive.ObjectID `json:"id" bson:"_id"`                                      // comment or reply ID
	ProfileID   primitive.ObjectID `json:"profileId,omitempty" bson:"profileId,omitempty"`     // required for comments
	ProfileType *string            `json:"profileType,omitempty" bson:"profileType,omitempty"` // required for comments
	//CreatedTS    time.Time          `json:"createdTS" bson:"-"`                       // extracted from OID
	CreatedID    primitive.ObjectID `json:"createdID" bson:"createdID"`
	CreatedName  string             `json:"createdName" bson:"createdName"`
	ModifiedTS   *time.Time         `json:"modifiedTS,omitempty" bson:"modifiedTS,omitempty"` // edited if present
	ModifiedID   primitive.ObjectID `json:"modifiedID,omitempty" bson:"modifiedID,omitempty"` // maybe used to flag "edited by admin"
	ModifiedName *string            `json:"modifiedName,omitempty" bson:"modifiedName,omitempty"`
	UpVotes      int32              `json:"upVotes" bson:"upVotes"`
	DownVotes    int32              `json:"downVotes" bson:"downVotes"`
	Rating       float32            `json:"rating" bson:"rating"`         // calculated by the voting function & persisted
	RatingSort   float32            `json:"ratingSort" bson:"ratingSort"` // calculated by the voting function & persisted (lowerBound)
	StatusCode   int32              `json:"statusCode" bson:"statusCD"`
	StatusText   string             `json:"statusText" bson:"-"`
	StatusTS     time.Time          `json:"statusTS" bson:"statusTS"`
	StatusID     primitive.ObjectID `json:"statusID" bson:"statusID"`
	StatusName   string             `json:"statusName" bson:"statusName"`
	Pinned       *bool              `json:"pinned,omitempty" bson:"pinned,omitempty"`
	Comment      string             `json:"comment" bson:"comment"`
	Replies      []Comment          `json:"replies,omitempty" bson:"replies,omitempty"` // applies to GET-requests only
}

// CommentListItem is the reduced data structure used for lists (eg. comment sections of profiles)
// this structure is NOT used for DB-access; instead data is copied from the "official" structure above
type CommentListItem struct {
	ID          primitive.ObjectID `json:"id"`
	CreatedTS   time.Time          `json:"createdTS"`
	CreatedID   primitive.ObjectID `json:"createdID"`
	CreatedName string             `json:"createdName"`
	Modified    bool               `json:"modified"`
	UpVotes     int32              `json:"upVotes"`
	DownVotes   int32              `json:"downVotes"`
	Pinned      *bool              `json:"pinned,omitempty"`
	Comment     string             `json:"comment"`
	Replies     []CommentListItem  `json:"replies,omitempty"`
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
		// insert new reply at the beginning of the array
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

// ListComments returns all comments and their possible answers to a given profile (limited)
func (m CommentModel) ListComments(profileId string, userID string) ([]CommentListItem, error) {

	id, err := primitive.ObjectIDFromHex(profileId)
	if err != nil {
		return nil, apperror.ErrNoData
	}

	// only read required fields for small list
	fields := bson.D{
		{Key: "createdID", Value: 1},
		{Key: "createdName", Value: 1},
		{Key: "modifiedTS", Value: 1},
		{Key: "upVotes", Value: 1},
		{Key: "downVotes", Value: 1},
		{Key: "pinned", Value: 1},
		{Key: "comment", Value: 1},
		{Key: "replies", Value: bson.D{
			{Key: "$slice", Value: 2}, // reads fist items, but full structure
		}},
		//{Key: "replies", Value: 1}, // reads full list, full structure
	}

	filter := bson.D{
		{Key: "profileId", Value: id},
	}

	sort := bson.D{
		{Key: "_id", Value: -1},
	}

	opts := options.Find().SetProjection(fields).SetLimit(5).SetSort(sort)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel() // nach 10 Sekunden abbrechen

	cursor, err := m.Collection.Find(ctx, filter, opts)
	if err != nil {
		return nil, helpers.WrapError(err, helpers.FuncName())
	}

	// receive results (full structure)
	var comments []Comment

	err = cursor.All(ctx, &comments)
	if err != nil {
		return nil, helpers.WrapError(err, helpers.FuncName())
	}

	// check for empty result set (no error raised by find)
	if comments == nil {
		return nil, apperror.ErrNoData
	}

	// copy data to reduced list-struct
	var commentList []CommentListItem
	var comment CommentListItem

	for _, c := range comments {
		comment.ID = c.ID
		comment.CreatedTS = primitive.ObjectID.Timestamp(c.ID)
		comment.CreatedID = c.CreatedID
		comment.CreatedName = c.CreatedName
		comment.Modified = (c.ModifiedTS != nil)
		comment.UpVotes = c.UpVotes
		comment.DownVotes = c.DownVotes
		comment.Pinned = c.Pinned
		comment.Comment = c.Comment
		if len(c.Replies) > 0 {
			comment.Replies = make([]CommentListItem, len(c.Replies))
			for i, r := range c.Replies {
				comment.Replies[i].ID = r.ID
				comment.Replies[i].CreatedTS = primitive.ObjectID.Timestamp(r.ID)
				comment.Replies[i].CreatedID = r.CreatedID
				comment.Replies[i].CreatedName = r.CreatedName
				comment.Replies[i].Modified = (r.ModifiedTS != nil)
				comment.Replies[i].UpVotes = r.UpVotes
				comment.Replies[i].DownVotes = r.DownVotes
				comment.Replies[i].Pinned = nil // by convention not present for replies
				comment.Replies[i].Comment = r.Comment
			}
		}

		commentList = append(commentList, comment)
	}

	// ToDO:
	// if userID != "" die Votes für die entsprechenden ListComments lesen (1x query dann in der Liste ergänzen)

	return commentList, nil
}
