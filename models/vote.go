package models

import (
	"context"
	"forza-garage/helpers"
	"math"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// validator (tags) used by Gin => https://github.com/go-playground/validator

// Vote represents a single vote action
type Vote struct {
	ProfileID primitive.ObjectID `json:"profileID" bson:"profileID" binding:"required"`
	UserID    primitive.ObjectID `json:"userID" bson:"userID"` // actually required, read from token
	UserName  string             `json:"userName" bson:"-"`
	VoteTS    time.Time          `json:"voteTS" bson:"voteTS"`                 // stored separately because users can change their vote
	Vote      int32              `json:"vote" bson:"vote" validate:"required"` // https://github.com/go-playground/validator/issues/290
}

// ProfileVotes represents the current state of votes related to a profile
type ProfileVotes struct {
	UpVotes   int32 `json:"upVotes"`
	DownVotes int32 `json:"downVotes"`
	UserVote  int32 `json:"userVote"` // vote action of the requested user (read from token)
}

const (
	VoteUp      int32 = 1
	VoteDown    int32 = -1
	VoteNeutral int32 = 0 // revoked or not voted
)

type VoteModel struct {
	Collection *mongo.Collection
	// Gewisse Informationen kommen vom User-Model, die werden hier referenziert
	// somit muss das nicht der Controller machen
	GetUserNameOID func(ID primitive.ObjectID) (string, error)
}

// CastVotes is used to vote for/against something (a profile, eg. Course/Championship)
// It also calcalutes the new rating and lower boundary to sort the profiles
func (v VoteModel) CastVote(profileOID primitive.ObjectID, userID string, vote int32, SetRating func(courseOID primitive.ObjectID, rating float32, sortOrder float32) error) error {

	// Positive | Negative votes will be Upserts
	// Revokes will be Deletes

	// Keine Prüfung, ob das ObjectID gültig ist. (dann braucht's den Typ als Parameter und alle COllections :-/)

	userOID := ObjectID(userID) // ToDo: auf err umstellen (?)

	// 1. save or delete vote
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

	// 2. calculate the new rating and sort order of the profile
	// reasons for client-side/api implemenation:
	// 1. speed
	// 2. complexity of queries

	// https://github.com/omsec/racing-db/blob/master/setup.sql
	// #441

	up, down, err := v.countVotes(profileOID)
	if err != nil {
		return err
	}

	// https://yourbasic.org/golang/round-float-to-int/

	var rating float32 = 0
	var ratingSort float32 = 0

	upVotes := float64(up)
	downVotes := float64(down)
	totalVotes := upVotes + downVotes

	if upVotes > 0 && totalVotes > 0 {
		rating = float32(math.Round(float64((((upVotes/totalVotes)*4)+1)*2) / 2))
		ratingSort = float32((upVotes+1.9208)/totalVotes - 1.96*math.Sqrt((upVotes*downVotes)/totalVotes+0.9604)/totalVotes/(1+3.8416/totalVotes)) // lower bound
	}

	SetRating(profileOID, rating, ratingSort)

	return nil
}

// GetVotes returns the up and down votes as well as the vote of the user
func (v VoteModel) GetVotes(profileID string, userID string) (profileVotes *ProfileVotes, err error) {

	profileOID := ObjectID(profileID)
	userOID := ObjectID(userID)
	profileVotes = new(ProfileVotes)

	// 1. get the user's vote
	if userID != "" {
		filter := bson.D{
			{Key: "profileID", Value: profileOID},
			{Key: "userID", Value: userOID},
		}

		fields := bson.D{
			{Key: "_id", Value: 0}, // _id kommt immer, daher explizit ausschalten
			{Key: "vote", Value: 1},
		}

		opts := options.FindOne().SetProjection(fields)

		// user vote
		data := struct {
			Vote int32 `bson:"vote"`
		}{VoteNeutral}

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel() // nach 10 Sekunden abbrechen

		err = v.Collection.FindOne(ctx, filter, opts).Decode(&data)
		if err != nil {
			// it's NOT an error if the user didn't vote
			if err != mongo.ErrNoDocuments {
				return nil, helpers.WrapError(err, helpers.FuncName())
			}
		}
		profileVotes.UserVote = data.Vote
	} else {
		profileVotes.UserVote = VoteNeutral
	}

	// 2. count votes for/against profile
	profileVotes.UpVotes, profileVotes.DownVotes, err = v.countVotes(profileOID)

	return profileVotes, nil
}

// count the actual votes for/against a profile
func (v VoteModel) countVotes(profileOID primitive.ObjectID) (up int32, down int32, err error) {

	matchStage := bson.D{
		{Key: "$match", Value: bson.D{
			{Key: "$and", Value: bson.A{
				bson.D{{Key: "profileID", Value: profileOID}},
			}},
		}},
	}

	// https://stackoverflow.com/questions/23116330/mongodb-select-count-group-by
	groupStage := bson.D{
		{Key: "$group", Value: bson.D{
			{Key: "_id", Value: "$vote"}, // values of "votes" (up/down action)
			{Key: "count", Value: bson.D{
				{Key: "$sum", Value: 1},
			},
			}},
		}}

	// ToDo: Hint (to use index)
	// https://www.unitconverters.net/time/second-to-nanosecond.htm
	opts := options.Aggregate().SetMaxTime(5000000000) // 5 secs

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel() // nach 10 Sekunden abbrechen

	cursor, err := v.Collection.Aggregate(ctx, mongo.Pipeline{
		matchStage,
		groupStage}, opts)
	if err != nil {
		return VoteNeutral, VoteNeutral, helpers.WrapError(err, helpers.FuncName())
	}

	var votes []bson.M
	err = cursor.All(ctx, &votes)
	if err != nil {
		// it's NOT an error if there are no votes at all
		if err != mongo.ErrNoDocuments {
			return VoteNeutral, VoteNeutral, helpers.WrapError(err, helpers.FuncName())
		}
	}

	// slice contains a map with values of "_id" and the field "count"
	for _, v := range votes {
		switch v["_id"].(int32) {
		case 1:
			up = v["count"].(int32)
		case -1:
			down = v["count"].(int32)
		}
	}

	return up, down, nil
}
