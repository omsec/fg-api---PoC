package authorization

import (
	"context"
	"forza-garage/apperror"
	"forza-garage/helpers"
	"forza-garage/lookups"
	"sort"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// Functions to check permissions
// without dependencies to the User Model

type Credentials struct {
	UserID       primitive.ObjectID
	LoginName    string
	RoleCode     int32 `bson:"roleCD"`
	LanguageCode int32 `bson:"languageCD"` // ToDo: Lesen aus Header für ANONYM, DB für Members (override-Möglichkeit)
	Friends      []UserRef
	userCol      *mongo.Collection
	socialCol    *mongo.Collection
}

// UserRef is a simple reference to something (another user as a friend or follower) or an object as an "observable"
type UserRef struct {
	UserID        primitive.ObjectID `json:"userID" bson:"userID"` // referencing user
	UserName      string             `json:"userName" bson:"userName"`
	ReferenceID   primitive.ObjectID `json:"referenceID" bson:"refID"`     // referenced ID
	ReferenceName string             `json:"referenceName" bson:"refName"` // name of referenced user/object
	ReferenceType string             `json:"referenceType" bson:"refType"` // user, course, championship etc.
	// eigentlich hier unnötig, aber einfacher
	RelationType string `json:"-" bson:"relType"` // friend, following/observing, follower
}

// SetConnections is called in Env Model Initializiation
func (c *Credentials) SetConnections(mongoCollections map[string]*mongo.Collection) {
	c.userCol = mongoCollections["users"]
	c.socialCol = mongoCollections["social"]
}

// GetCredentials returns account infos to control permissions and text-out (language)
// any error is considered an anonymous user (visitor) to public items
func (c *Credentials) GetCredentials(userOID primitive.ObjectID, loadFriendlist bool) *Credentials {
	var credentials Credentials

	fields := bson.D{
		{Key: "_id", Value: 0}, // _id kommt immer, ausser es wird explizit ausgeschlossen (0)
		{Key: "loginName", Value: 1},
		{Key: "XBoxTag", Value: 1},
		{Key: "roleCD", Value: 1}, // {Key: "metaInfo.rating", Value: 1}, -- so könnte die nested struct eingeschränkt werden
		{Key: "languageCD", Value: 1},
		{Key: "privacyCD", Value: 1},
	}

	opts := options.FindOne().SetProjection(fields)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel() // nach 10 Sekunden abbrechen

	err := c.userCol.FindOne(ctx, bson.M{"_id": userOID}, opts).Decode(&credentials)
	if err != nil {
		c.setDefaultProfile(&credentials)
	}
	credentials.UserID = userOID // not read again from DB ;-)

	// friendlist ist referenced from its own collection, add it
	if loadFriendlist {
		credentials.Friends, _ = c.getReferences(userOID, "friend")
		// error checking removed, since the user is already checked, even in case of an error
		/*
			if err != nil {
				// "no data/no friends" is not an error, other errors lead to anonymous user
				if err != apperror.ErrNoData {
					m.setDefaultProfile(&credentials)
				}
			}
		*/
	}

	return &credentials
}

// this is used as the error handler of GetCredentials
// any error of that function will be threated as an anonymous user, receiving the default credentials
func (c *Credentials) setDefaultProfile(credentials *Credentials) {
	credentials.UserID = primitive.NilObjectID
	credentials.RoleCode = lookups.UserRoleGuest
	// ToDO: Lang passed via Browser
}

// private proc to read relations/referenced documents, such as friends
func (c *Credentials) getReferences(userOID primitive.ObjectID, relationType string) ([]UserRef, error) {

	fields := bson.M{
		"_id":      0,
		"userID":   1,
		"userName": 1,
		"refID":    1,
		"refName":  1,
	}

	// actually not required for friendlist, due do post-processing (sort on slice)
	dbSort := bson.M{
		"userName": 1,
	}

	// ToDo: Limit sinnvoll?
	opts := options.Find().SetProjection(fields).SetLimit(20).SetSort(dbSort)

	// different query depending on relation type
	var filter bson.M

	switch relationType {
	case "blocking":
		// user A has blocked user B
		filter = bson.M{
			"relType": relationType,
			"userID":  userOID,
		}
	case "friend":
		filter = bson.M{
			"relType": relationType,
			"$or": bson.A{
				bson.M{"userID": userOID},
				bson.M{"refID": userOID},
			},
		}
	case "following":
		// wem folge ich? abfrage auf db.userID = userID
		filter = bson.M{
			"relType": relationType,
			"userID":  userOID,
		}
	case "follower":
		// wer folgt mir? abfrage auf refID = userID
		filter = bson.M{
			"relType": "following", // same type/verb, different context
			"refID":   userOID,
		}
	case "observing":
		// welche rat/cmp etc. beobachte ich?
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel() // nach 10 Sekunden abbrechen

	cursor, err := c.socialCol.Find(ctx, filter, opts)
	if err != nil {
		return nil, helpers.WrapError(err, helpers.FuncName())
	}

	// receive results
	var results []UserRef

	err = cursor.All(ctx, &results)
	if err != nil {
		return nil, helpers.WrapError(err, helpers.FuncName())
	}

	// check for empty result set (no error raised by find)
	if results == nil {
		return nil, apperror.ErrNoData
	}

	// final list
	var reference UserRef
	var references []UserRef
	// storing 2 docs and use a trx makes it easier, but takes more disk space (=pay!)
	// if userID = res.userID -> res.refID
	// if userID = res.refID -> res.userID

	if relationType == "friend" {
		for _, r := range results {
			reference.UserID = userOID
			if r.UserID == userOID {
				reference.UserName = r.UserName
				reference.ReferenceID = r.ReferenceID
				reference.ReferenceName = r.ReferenceName
			} else {
				reference.UserName = r.ReferenceName
				reference.ReferenceID = r.UserID
				reference.ReferenceName = r.UserName
			}
			reference.ReferenceType = "user"
			reference.ReferenceType = relationType

			references = append(references, reference)
		}
		// https://zetcode.com/golang/sort/
		sort.Slice(references, func(i, j int) bool {
			return references[i].ReferenceName < references[j].ReferenceName
		})
	}

	if relationType == "following" {
		for _, r := range results {
			reference.UserID = r.UserID
			reference.UserName = r.UserName
			reference.ReferenceID = r.ReferenceID
			reference.ReferenceName = r.ReferenceName
			reference.ReferenceType = "user"
			reference.RelationType = relationType

			references = append(references, reference)
		}
	}

	if relationType == "follower" {
		for _, r := range results {
			reference.UserID = r.ReferenceID
			reference.UserName = r.ReferenceName
			reference.ReferenceID = r.UserID
			reference.ReferenceName = r.UserName
			reference.ReferenceType = "user"
			reference.RelationType = relationType

			references = append(references, reference)
		}
	}

	return references, nil
}
