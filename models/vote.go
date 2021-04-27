package models

import (
	"context"
	"forza-garage/apperror"
	"forza-garage/helpers"
	"math"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// vote (action) type
const (
	VoteUp      int32 = 1
	VoteDown    int32 = -1
	VoteNeutral int32 = 0 // revoked or not voted
)

// validator (tags) used by Gin => https://github.com/go-playground/validator

// Vote represents a single vote action
type Vote struct {
	// ID ommitted, yet created in document
	ProfileID primitive.ObjectID `json:"profileID" bson:"profileID" binding:"required"`
	// some items are displayed in lists, eg. comments
	// to speed up querying (by reducing returned data) of user actions, the object type is stored
	ProfileType string             `json:"profileType" bson:"profileType" binding:"required"`
	UserID      primitive.ObjectID `json:"userID" bson:"userID"` // actually required, read from token
	UserName    string             `json:"userName" bson:"-"`
	VoteTS      time.Time          `json:"voteTS" bson:"voteTS"`                 // stored separately because users can change their vote
	Vote        int32              `json:"vote" bson:"vote" validate:"required"` // https://github.com/go-playground/validator/issues/290
}

// ProfileVotes represents the current state of votes related to a profile
type ProfileVotes struct {
	UpVotes   int32 `json:"upVotes"`
	DownVotes int32 `json:"downVotes"`
	UserVote  int32 `json:"userVote"` // vote action of the requested user (read from token)
}

// UserVote represents a user's vote actions to a profile
// usually used as a slice type for lists
type UserVote struct {
	ProfileID primitive.ObjectID `json:"profileId"`
	UserVote  int32              `json:"userVote" bson:"vote"` // primitive values need bson tag
}

// VoteModel provides the logics to the data type
type VoteModel struct {
	Collection *mongo.Collection
	// Gewisse Informationen kommen vom User-Model, die werden hier referenziert
	// somit muss das nicht der Controller machen
	GetUserNameOID func(ID primitive.ObjectID) (string, error)
}

// CastVotes is used to vote for/against something (a profile, eg. Course/Championship)
// It also calcalutes the new rating and lower boundary to sort the profiles
func (v VoteModel) CastVote(
	vote Vote,
	SetRating func(social *Social) error) (profileVotes *ProfileVotes, err error) {

	// Positive | Negative votes will be Upserts
	// Revokes will be Deletes

	// Keine Prüfung, ob das ObjectID gültig ist. (dann braucht's alle COllections :-/)

	// 1. save or delete vote
	if vote.Vote != VoteNeutral {
		usr, err := v.GetUserNameOID(vote.UserID)
		if err != nil {
			return nil, ErrInvalidUser
		}

		filter := bson.D{
			{Key: "profileID", Value: vote.ProfileID},
			{Key: "userID", Value: vote.UserID},
		}

		fields := bson.D{
			{Key: "$set", Value: bson.D{{Key: "profileID", Value: vote.ProfileID}}},
			{Key: "$set", Value: bson.D{{Key: "profileType", Value: vote.ProfileType}}},
			{Key: "$set", Value: bson.D{{Key: "userID", Value: vote.UserID}}},
			{Key: "$set", Value: bson.D{{Key: "userName", Value: usr}}},
			{Key: "$set", Value: bson.D{{Key: "voteTS", Value: time.Now()}}}, // $currentDate müsste nochmal "verpackt" werden
			{Key: "$set", Value: bson.D{{Key: "vote", Value: vote.Vote}}},
		}

		opts := options.Update().SetUpsert(true)

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel() // nach 10 Sekunden abbrechen

		// not interessted in actual result
		_, err = v.Collection.UpdateOne(ctx, filter, fields, opts)
		if err != nil {
			return nil, helpers.WrapError(err, helpers.FuncName())
		}

	} else {
		// delete vote (revoke)
		filter := bson.D{
			{Key: "profileID", Value: vote.ProfileID},
			{Key: "userID", Value: vote.UserID},
		}

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel() // nach 10 Sekunden abbrechen

		// not interessted in actual result
		_, err := v.Collection.DeleteOne(ctx, filter)
		if err != nil {
			return nil, helpers.WrapError(err, helpers.FuncName())
		}
	}

	// 2. calculate the new rating and sort order of the profile
	// reasons for client-side/api implemenation:
	//  I. speed
	// II. complexity of queries

	// https://github.com/omsec/racing-db/blob/master/setup.sql
	// #441

	// ToDo:
	// überlegen, ob hier wirklich agregiert werden soll/kann (performance)
	up, down, err := v.countVotes(vote.ProfileID)
	if err != nil {
		return nil, err
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

	// pass "social meta data" to the referenced profile
	// which will stored it in its document

	// the struct literal syntax also creates the variable
	// https://yourbasic.org/golang/structs-explained/
	social := &Social{
		ProfileOID: vote.ProfileID,
		Rating:     rating,
		SortOrder:  ratingSort,
		UpVotes:    up,
		DownVotes:  down,
		TouchedTS:  time.Now(),
	}

	SetRating(social)

	profileVotes = new(ProfileVotes)
	profileVotes.DownVotes = down
	profileVotes.UpVotes = up
	profileVotes.UserVote = vote.Vote

	return profileVotes, nil
}

// GetUserVotes returns the vote action of a user
func (v VoteModel) GetUserVote(profileID string, userID string) (int32, error) {

	profileOID := helpers.ObjectID(profileID)
	userOID := helpers.ObjectID(userID)

	// 1. get the user's vote

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

	err := v.Collection.FindOne(ctx, filter, opts).Decode(&data)
	if err != nil {
		// it's NOT an error if the user didn't vote
		if err != mongo.ErrNoDocuments {
			return VoteNeutral, helpers.WrapError(err, helpers.FuncName())
		}
	}
	return data.Vote, nil
}

// GetUserVotes returns the vote actions of a user to objects of a specific type
// usually used for items displayed in lists, such as comments
func (v VoteModel) GetUserVotes(domain string, userID string) ([]UserVote, error) {

	userOID := helpers.ObjectID(userID)

	fields := bson.D{
		{Key: "_id", Value: 0}, // _id kommt immer, ausser es wird explizit ausgeschlossen (0)
		{Key: "profileID", Value: 1},
		{Key: "vote", Value: 1},
	}

	filter := bson.D{
		{Key: "userID", Value: userOID},
		{Key: "profileType", Value: domain},
	}

	opts := options.Find().SetProjection(fields).SetLimit(20)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel() // nach 10 Sekunden abbrechen

	cursor, err := v.Collection.Find(ctx, filter, opts)
	if err != nil {
		return nil, helpers.WrapError(err, helpers.FuncName())
	}

	// receive results
	var votes []UserVote

	err = cursor.All(ctx, &votes)
	if err != nil {
		return nil, helpers.WrapError(err, helpers.FuncName())
	}

	// check for empty result set (no error raised by find)
	if votes == nil {
		return nil, apperror.ErrNoData
	}

	return votes, nil
}

// GetVotes returns the up and down votes as well as the vote of the user
// zur Zeit unbenutzt (gelesen über parent's meta); evtl. mal für stats-page
/*
func (v VoteModel) GetVotes(profileID string, userID string) (profileVotes *ProfileVotes, err error) {

	profileOID := helpers.ObjectID(profileID)
	userOID := helpers.ObjectID(userID)
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
*/

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
