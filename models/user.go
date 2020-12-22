package models

import (
	"context"
	"errors"
	"forza-garage/database"
	"forza-garage/helpers"
	"forza-garage/lookups"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// User is the "interface" used for client communication
type User struct {
	ID           primitive.ObjectID `json:"id" bson:"_id"`
	LoginName    string             `json:"loginName" bson:"loginName"`
	Password     string             `json:"password" bson:"password"` // hash value
	RoleCode     string             `json:"roleCode" bson:"roleCD"`
	RoleText     string             `json:"roleText" bson:"-"`
	LanguageCode string             `json:"languageCode" bson:"languageCD"`
	LanguageText string             `json:"languageText" bson:"-"`
	EMailAddress string             `json:"eMail" bson:"eMail"`
	XBoxTag      string             `json:"XBoxTag" bson:"XBoxTag"`
	LastSeenTS   time.Time          `json:"lastSeenTS" bson:"lastSeenTS,omitempty"`
}

// UserModel provides the logic to the interface and access to the database
type UserModel struct {
	Client     *mongo.Client
	Collection *mongo.Collection
}

// custom error types - evtl in eigenes file
var (
	ErrUserNameNotAvailable = errors.New("user name is not available")
	ErrEMailAddressTaken    = errors.New("email-address is already used")
	ErrInvalidUser          = errors.New("invalid user name or password")
	ErrInvalidPassword      = errors.New("password does not meet rules")
)

// UserExists checks if a User Name is available - used in client for in-type error checking
// (wrapper of internal helper function)
func (m UserModel) UserExists(userName string) bool {
	b, _ := userExists(m.Collection, userName)
	return b
}

// EMailAddressTaken checks if an eMail-Address is already assigned with any User Name
func (m UserModel) EMailAddressTaken(emailAddress string) bool {
	b, _ := emailTaken(m.Collection, emailAddress)
	return b
}

// CreateUser adds a new User
func (m UserModel) CreateUser(user User) (string, error) {

	var err error

	// ToDO: Validate (ext fnc)

	b, err := userExists(m.Collection, user.LoginName)
	if b || err != nil {
		return "", ErrUserNameNotAvailable
	}

	b, err = emailTaken(m.Collection, user.EMailAddress)
	if b || err != nil {
		return "", ErrEMailAddressTaken
	}

	// ToDO: move to validate proc
	pwdHash, err := helpers.GenerateHash(user.Password)
	if err != nil {
		return "", err
	}

	user.ID = primitive.NewObjectID()
	user.Password = pwdHash
	user.RoleCode = lookups.UserRole(lookups.URguest)
	user.LastSeenTS = time.Now()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel() // nach 10 Sekunden abbrechen

	res, err := m.Collection.InsertOne(ctx, user)
	if err != nil {
		return "", err // primitive.NilObjectID.Hex() ? probly useless
	}

	return res.InsertedID.(primitive.ObjectID).Hex(), nil
}

// GetUserByName reads a user's login account data
func (m UserModel) GetUserByName(userName string) (*User, error) {

	var err error
	var user User

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel() // nach 10 Sekunden abbrechen

	err = m.Collection.FindOne(ctx, bson.M{"loginName": userName}).Decode(&user)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, ErrInvalidUser
		}
		// pass any other real
		return nil, err
	}

	// add look-up texts
	addLookups(&user)

	return &user, nil
}

// GetUserByID reads a user's login account data
func (m UserModel) GetUserByID(ID string) (*User, error) {

	var user User

	// https://ildar.pro/golang-hints-create-mongodb-object-id-from-string/
	id, err := primitive.ObjectIDFromHex(ID)
	if err != nil {
		return nil, ErrInvalidUser
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel() // nach 10 Sekunden abbrechen

	err = m.Collection.FindOne(ctx, bson.M{"_id": id}).Decode(&user)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, ErrInvalidUser
		}
		// pass any other real
		return nil, err
	}

	// add look-up text
	//user.RoleText = database.GetLookupText(lookups.LookupType(lookups.LTuserRole), user.RoleCode)
	addLookups(&user)

	return &user, nil
}

// GetUserName returns the login name from an ID (reduced version, without profile data)
func (m UserModel) GetUserName(ID string) (string, error) {

	data := struct {
		//ID        primitive.ObjectID `bson:"_id"`
		LoginName string `bson:"loginName"`
	}{}

	id, err := primitive.ObjectIDFromHex(ID)
	if err != nil {
		return "", ErrInvalidUser
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel() // nach 10 Sekunden abbrechen

	fields := bson.D{
		{Key: "_id", Value: 0}, // _id kommt immer, ausser es wird explizit ausgeschlossen (0)
		{Key: "loginName", Value: 1}}

	err = m.Collection.FindOne(ctx, bson.M{"_id": id}, options.FindOne().SetProjection(fields)).Decode(&data)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return "", ErrInvalidUser
		}
		// pass any other error
		return "", err
	}

	return data.LoginName, nil
}

// CheckPassword tests if a login's password matches
// (kein DB-Zugriff nötig)
func (m UserModel) CheckPassword(givenPassword string, userInfo User) bool {
	match, err := helpers.CompareHash(userInfo.Password, givenPassword)
	if err != nil {
		return false
	}
	return match
}

// SetLastSeen saves timestamp of last log-in
// ToDo: add IP-Address & record history (collection analytics)
func (m UserModel) SetLastSeen(userID primitive.ObjectID) {
	// no error is returned since this function is not essential

	filter := bson.D{{Key: "_id", Value: userID}}
	update := bson.D{{Key: "$set", Value: bson.D{{Key: "lastSeenTS", Value: time.Now()}}}}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel() // nach 10 Sekunden abbrechen

	// just fire & forget
	_, _ = m.Collection.UpdateOne(ctx, filter, update)
}

// SetPassword is used to change a User's password
func (m UserModel) SetPassword(userID primitive.ObjectID, newPassword string) error {
	// ToDO: call PWD-Validator func

	pwdHash, err := helpers.GenerateHash(newPassword)
	if err != nil {
		return err
	}

	filter := bson.D{{Key: "_id", Value: userID}}
	update := bson.D{{Key: "$set", Value: bson.D{{Key: "password", Value: pwdHash}}}}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel() // nach 10 Sekunden abbrechen

	result, err := m.Collection.UpdateOne(ctx, filter, update)
	if err != nil {
		return err
	}

	// just an additional check to discover data consistency problems
	if result.MatchedCount != 1 || result.ModifiedCount != 1 {
		return errors.New("mulitple records found")
	}

	return nil
}

// internal implementations that are used by multiple methods of the model and corresponding handlers
func userExists(collection *mongo.Collection, userName string) (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel() // nach 10 Sekunden abbrechen

	// there seems to be no function like "exists" so a projection on just the ID is used
	fields := bson.D{
		{Key: "_id", Value: 1}}

	data := struct {
		ID primitive.ObjectID `bson:"_id"`
	}{}

	// some (old) sources say FindOne is slow and we should use find instead... (?)
	err := collection.FindOne(ctx, bson.M{"loginName": userName}, options.FindOne().SetProjection(fields)).Decode(&data)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return false, nil
		}
		// treat errors as a "yes" - caller should not evaluate the result in case of an error
		return true, err
	}
	// no error means a document was found, hence the user does exist
	return true, nil
}

func emailTaken(collection *mongo.Collection, emailAddress string) (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel() // nach 10 Sekunden abbrechen

	// there seems to be no function like "exists" so a projection on just the ID is used
	fields := bson.D{
		{Key: "_id", Value: 1}}

	data := struct {
		ID primitive.ObjectID `bson:"_id"`
	}{}

	// some (old) sources say FindOne is slow and we should use find instead... (?)
	// ToDO: Add index to field in MongoDB
	err := collection.FindOne(ctx, bson.M{"eMail": emailAddress}, options.FindOne().SetProjection(fields)).Decode(&data)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return false, nil
		}
		// treat errors as a "yes" - caller should not evaluate the result in case of an error
		return true, err
	}
	// no error means a document was found, hence the user does exist
	return true, nil
}

// internal helpers
// actually that's not immutable, but ok here
func addLookups(user *User) *User {
	user.RoleText = database.GetLookupText(lookups.LookupType(lookups.LTuserRole), user.RoleCode)
	user.LanguageText = database.GetLookupText(lookups.LookupType(lookups.LTlang), user.LanguageCode)

	return user
}
