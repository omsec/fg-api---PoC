package models

import (
	"context"
	"forza-garage/helpers"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// validator (tags) used by Gin => https://github.com/go-playground/validator
type Vote struct {
	ProfileID primitive.ObjectID `json:"profileID" bson:"profileID" binding:"required"`
	UserID    primitive.ObjectID `json:"userID" bson:"userID"` // actually required, read from token
	UserName  string             `json:"userName" bson:"-"`
	VoteTS    time.Time          `json:"voteTS" bson:"voteTS"`                 // stored separately because users can change their vote
	Vote      int                `json:"vote" bson:"vote" validate:"required"` // https://github.com/go-playground/validator/issues/290
}

const (
	VoteUp      int = 1
	VoteDown    int = -1
	VoteNeutral int = 0 // revoked or not voted
)

type VoteModel struct {
	Collection *mongo.Collection
	// Gewisse Informationen kommen vom User-Model, die werden hier referenziert
	// somit muss das nicht der Controller machen
	GetUserNameOID func(ID primitive.ObjectID) (string, error)
}

func (v VoteModel) Vote(profileOID primitive.ObjectID, userID string, vote int) error {

	// Positive | Negative votes will be Upserts
	// Revokes will be Deletes

	// TODO: Evt. Rating & Sort Order hier berechnen und im Object-Doc speichern

	// ToDo: Prüfen, ob das ObjectID gültig ist? (dann braucht's den Typ als Parameter und alle COllections :-/)

	userOID := ObjectID(userID) // ToDo: auf err umstellen (?)

	if vote != VoteNeutral {
		usr, err := v.GetUserNameOID(userOID)
		if err != nil {
			return ErrInvalidUser
		}

		filter := bson.D{
			{Key: "profileID", Value: profileOID},
			{Key: "userID", Value: userOID},
		}

		fields := bson.D{
			{Key: "$set", Value: bson.D{{Key: "profileID", Value: profileOID}}},
			{Key: "$set", Value: bson.D{{Key: "userID", Value: userOID}}},
			{Key: "$set", Value: bson.D{{Key: "userName", Value: usr}}},
			{Key: "$set", Value: bson.D{{Key: "voteTS", Value: time.Now()}}}, // $currentDate müsste nochmal "verpackt" werden
			{Key: "$set", Value: bson.D{{Key: "vote", Value: vote}}},
		}

		opts := options.Update().SetUpsert(true)

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel() // nach 10 Sekunden abbrechen

		// not interessted in actual result
		_, err = v.Collection.UpdateOne(ctx, filter, fields, opts)
		if err != nil {
			return helpers.WrapError(err, helpers.FuncName())
		}

	} else {
		// delete vote (revoke)
		filter := bson.D{
			{Key: "profileID", Value: profileOID},
			{Key: "userID", Value: userOID},
		}

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel() // nach 10 Sekunden abbrechen

		// not interessted in actual result
		_, err := v.Collection.DeleteOne(ctx, filter)
		if err != nil {
			return helpers.WrapError(err, helpers.FuncName())
		}

	}

	return nil
}

func (v VoteModel) GetVote(objectID string, userID string) (int, error) {
	return VoteNeutral, nil
}
